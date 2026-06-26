package server

import (
	"context"
	"sync"
)

// backtestRunManager tracks the in-flight simulation cancel func (one run at a time).
type backtestRunManager struct {
	mu     sync.Mutex
	gen    uint64
	cancel context.CancelFunc
}

func newBacktestRunManager() *backtestRunManager {
	return &backtestRunManager{}
}

func (m *backtestRunManager) begin(parent context.Context) (context.Context, func()) {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.gen++
	runGen := m.gen
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.mu.Unlock()

	return ctx, func() {
		m.mu.Lock()
		if m.gen == runGen {
			m.cancel = nil
		}
		m.mu.Unlock()
		cancel()
	}
}

func (m *backtestRunManager) stop() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel == nil {
		return false
	}
	m.cancel()
	return true
}
