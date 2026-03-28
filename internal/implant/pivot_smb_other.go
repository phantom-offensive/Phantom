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
	return nil, fmt.Errorf("pivot not supported on this platform")
}
