package listener

import (
	"fmt"
	"sync"
)

// Manager manages the lifecycle of all listeners.
type Manager struct {
	mu        sync.RWMutex
	listeners map[string]*HTTPListener // name -> listener
}

// NewManager creates a new listener manager.
func NewManager() *Manager {
	return &Manager{
		listeners: make(map[string]*HTTPListener),
	}
}

// Add registers a listener (does not start it).
func (m *Manager) Add(l *HTTPListener) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.listeners[l.Name]; exists {
		return fmt.Errorf("listener %q already exists", l.Name)
	}

	m.listeners[l.Name] = l
	return nil
}

// Start starts a listener by name in a goroutine.
func (m *Manager) Start(name string) error {
	m.mu.RLock()
	l, exists := m.listeners[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("listener %q not found", name)
	}

	if l.IsRunning() {
		return fmt.Errorf("listener %q is already running", name)
	}

	go func() {
		if err := l.Start(); err != nil {
			l.emitEvent("listener_error", name, err.Error())
		}
	}()

	return nil
}

// Stop stops a listener by name.
func (m *Manager) Stop(name string) error {
	m.mu.RLock()
	l, exists := m.listeners[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("listener %q not found", name)
	}

	if !l.IsRunning() {
		return fmt.Errorf("listener %q is not running", name)
	}

	return l.Stop()
}

// Remove stops and removes a listener.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, exists := m.listeners[name]
	if !exists {
		return fmt.Errorf("listener %q not found", name)
	}

	if l.IsRunning() {
		l.Stop()
	}

	delete(m.listeners, name)
	return nil
}

// Get retrieves a listener by name.
func (m *Manager) Get(name string) (*HTTPListener, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.listeners[name]
	return l, ok
}

// List returns all registered listeners.
func (m *Manager) List() []*HTTPListener {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*HTTPListener, 0, len(m.listeners))
	for _, l := range m.listeners {
		list = append(list, l)
	}
	return list
}

// StopAll stops all running listeners.
func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, l := range m.listeners {
		if l.IsRunning() {
			l.Stop()
		}
	}
}
