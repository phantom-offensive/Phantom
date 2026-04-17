//go:build !windows && !linux

package implant

import "fmt"

type PipeRelay struct{}

func NewPipeRelay(name string) *PipeRelay { return &PipeRelay{} }

func StartPipeRelay(pipeName string) ([]byte, error) {
	return nil, fmt.Errorf("pivot relay not supported on this platform")
}

func StopPipeRelay() ([]byte, error) {
	return nil, fmt.Errorf("pivot relay not supported on this platform")
}

func ListPivots() ([]byte, error) {
	return nil, fmt.Errorf("pivot not supported on this platform")
}

func ExecutePivotCommand(args []string) ([]byte, error) {
	if len(args) == 0 {
		return []byte("Usage: pivot <start|stop|list|tcp-start|tcp-stop|tcp-list> [name/addr]"), nil
	}
	switch args[0] {
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
	case "start", "stop", "list":
		return []byte("SMB pivot requires Windows. Use tcp-start for cross-platform pivoting."), nil
	default:
		return []byte("Unknown pivot command"), nil
	}
}
