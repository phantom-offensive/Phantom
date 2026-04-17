package listener

import (
	"fmt"
	"sync"
)

// Listener is the interface implemented by all listener types (HTTP, WS, TCP, DNS…).
type Listener interface {
	GetID() string
	GetName() string
	GetType() string
	GetBindAddr() string
	Start() error
	Stop() error
	IsRunning() bool
}

// Manager manages the lifecycle of all listeners.
type Manager struct {
	mu        sync.RWMutex
	listeners map[string]Listener // name -> listener
}

// NewManager creates a new listener manager.
func NewManager() *Manager {
	return &Manager{
		listeners: make(map[string]Listener),
	}
}

// Add registers a listener (does not start it).
func (m *Manager) Add(l Listener) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := l.GetName()
	if _, exists := m.listeners[name]; exists {
		return fmt.Errorf("listener %q already exists", name)
	}

	m.listeners[name] = l
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
			// Emit via HTTPListener emitEvent if available, otherwise ignore
			if hl, ok := l.(*HTTPListener); ok {
				hl.emitEvent("listener_error", name, err.Error())
			} else if wl, ok := l.(*WSListener); ok {
				if wl.onEvent != nil {
					wl.onEvent("listener_error", name, err.Error())
				}
			}
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
func (m *Manager) Get(name string) (Listener, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.listeners[name]
	return l, ok
}

// List returns all registered listeners.
func (m *Manager) List() []Listener {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Listener, 0, len(m.listeners))
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
