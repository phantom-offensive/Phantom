// Package exchannel provides an External C2 (ExC2) plugin interface.
// ExC2 channels allow C2 communications over arbitrary third-party services
// (Slack, Teams, GitHub Gists, DNS-over-HTTPS, etc.) that are often
// allowed by corporate egress controls even when direct HTTP/HTTPS is blocked.
package exchannel

import "context"

// Channel is the interface all ExC2 channels must implement.
// The server polls each registered channel for inbound agent messages
// and delivers outbound tasks back through the same channel.
type Channel interface {
	// Name returns the channel identifier (e.g., "slack", "teams", "gist").
	Name() string
	// Start begins polling/listening on the channel.
	Start(ctx context.Context) error
	// Stop shuts down the channel.
	Stop() error
	// Send delivers a task payload to an agent via this channel.
	// agentID is used to address the correct agent-side listener.
	Send(agentID string, payload []byte) error
	// Recv blocks until an inbound message arrives from an agent,
	// then returns the agent ID and raw payload.
	Recv(ctx context.Context) (agentID string, payload []byte, err error)
	// IsRunning returns whether the channel is active.
	IsRunning() bool
}

// Registry holds all registered ExC2 channels.
type Registry struct {
	channels map[string]Channel
}

// NewRegistry creates an empty channel registry.
func NewRegistry() *Registry {
	return &Registry{channels: make(map[string]Channel)}
}

// Register adds a channel to the registry.
func (r *Registry) Register(ch Channel) {
	r.channels[ch.Name()] = ch
}

// Get returns a channel by name.
func (r *Registry) Get(name string) (Channel, bool) {
	ch, ok := r.channels[name]
	return ch, ok
}

// List returns all registered channel names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.channels))
	for n := range r.channels {
		names = append(names, n)
	}
	return names
}
