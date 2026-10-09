// Package audit provides a shared, in-memory operator audit log that is fed by
// both the CLI and the Web UI so a single trail records every operator action.
package audit

import (
	"sync"
	"time"
)

// Entry is a single operator action recorded in the audit trail.
type Entry struct {
	Time     string `json:"time"`
	Operator string `json:"operator"`
	Agent    string `json:"agent"`
	Action   string `json:"action"`
	Detail   string `json:"detail"`
}

const maxEntries = 500

var (
	mu      sync.Mutex
	entries []Entry
)

// Add records an operator action. The trail is bounded to the most recent
// maxEntries entries.
func Add(operator, agent, action, detail string) {
	mu.Lock()
	defer mu.Unlock()
	entries = append(entries, Entry{
		Time:     time.Now().Format("15:04:05"),
		Operator: operator,
		Agent:    agent,
		Action:   action,
		Detail:   detail,
	})
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
}

// List returns a snapshot of the audit trail, oldest first.
func List() []Entry {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Entry, len(entries))
	copy(out, entries)
	return out
}
