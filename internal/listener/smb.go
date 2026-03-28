package listener

// SMB Named Pipe Listener
//
// Enables agent-to-agent chaining over SMB named pipes.
// Only the "edge" agent needs internet access — internal agents
// communicate via named pipes through the compromised network.
//
// Architecture:
//   [C2 Server] ←HTTP/DNS→ [Edge Agent] ←SMB Pipe→ [Internal Agent]
//                                        ←SMB Pipe→ [Internal Agent]
//
// The edge agent acts as a relay:
// 1. Internal agent connects to \\edge-host\pipe\phantom
// 2. Edge agent forwards data to C2 server on next check-in
// 3. C2 response is relayed back through the pipe
//
// This is implemented as a task type rather than a separate listener,
// since the pipe runs on the agent side, not the server side.

// SMBPipeConfig holds configuration for the named pipe relay.
type SMBPipeConfig struct {
	PipeName    string // e.g., "phantom" → \\.\pipe\phantom
	MaxClients  int
	BufferSize  int
}

// DefaultSMBPipeConfig returns default pipe configuration.
func DefaultSMBPipeConfig() SMBPipeConfig {
	return SMBPipeConfig{
		PipeName:   "msupdate",    // Looks like a Windows Update pipe
		MaxClients: 10,
		BufferSize: 65536,
	}
}
