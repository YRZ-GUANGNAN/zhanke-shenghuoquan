package notification

import (
	"context"
	"sync"
	"time"
)

// WorkerState 描述一个后台投递 worker 的运行状态。
// 这些 goroutine 崩溃后不会自动重启，必须让运维在后台看到，而不是静默停止。
type WorkerState struct {
	Name       string    `json:"name"`
	Running    bool      `json:"running"`
	StartedAt  time.Time `json:"startedAt"`
	StoppedAt  time.Time `json:"stoppedAt,omitempty"`
	LastError  string    `json:"lastError"`
	ExitReason string    `json:"exitReason"`
}

// WorkerHealth 记录通知相关 worker 的生命周期，供管理后台展示。
type WorkerHealth struct {
	mu     sync.RWMutex
	states map[string]*WorkerState
	order  []string
}

func NewWorkerHealth() *WorkerHealth {
	return &WorkerHealth{states: map[string]*WorkerState{}}
}

// Run 在独立 goroutine 中启动 worker，并记录退出原因与错误。
func (h *WorkerHealth) Run(ctx context.Context, name string, worker func(context.Context) error) {
	if h == nil {
		go func() { _ = worker(ctx) }()
		return
	}
	state := &WorkerState{Name: name, Running: true, StartedAt: time.Now().UTC()}
	h.mu.Lock()
	if _, exists := h.states[name]; !exists {
		h.order = append(h.order, name)
	}
	h.states[name] = state
	h.mu.Unlock()

	go func() {
		err := worker(ctx)
		h.mu.Lock()
		state.Running = false
		state.StoppedAt = time.Now().UTC()
		if err != nil {
			state.LastError = err.Error()
			state.ExitReason = "error"
		} else {
			state.ExitReason = "stopped"
		}
		h.mu.Unlock()
	}()
}

func (h *WorkerHealth) Snapshot() []WorkerState {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	values := make([]WorkerState, 0, len(h.order))
	for _, name := range h.order {
		values = append(values, *h.states[name])
	}
	return values
}
