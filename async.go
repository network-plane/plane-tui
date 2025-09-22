package tui

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskFunc represents an asynchronous job.
type TaskFunc func(ctx context.Context, output OutputChannel) error

// TaskStatus enumerates async task states.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskSucceeded TaskStatus = "succeeded"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// TaskOptions configure async tasks.
type TaskOptions struct {
	Timeout  time.Duration
	Metadata map[string]any
}

// TaskHandle represents a running task.
type TaskHandle struct {
	ID       string
	Name     string
	Status   TaskStatus
	Error    error
	Metadata map[string]any
	cancel   context.CancelFunc
}

// TaskManager supervises background tasks.
type TaskManager struct {
	mu     sync.RWMutex
	seq    int
	tasks  map[string]*TaskHandle
	output OutputChannel
}

// NewTaskManager constructs a TaskManager.
func NewTaskManager(output OutputChannel) *TaskManager {
	return &TaskManager{tasks: map[string]*TaskHandle{}, output: output}
}

// Spawn launches an async task.
func (m *TaskManager) Spawn(name string, fn TaskFunc, opts TaskOptions) *TaskHandle {
	m.mu.Lock()
	m.seq++
	id := fmt.Sprintf("task-%d", m.seq)
	ctx, cancel := context.WithCancel(context.Background())
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}
	handle := &TaskHandle{
		ID:       id,
		Name:     name,
		Status:   TaskPending,
		Metadata: opts.Metadata,
		cancel:   cancel,
	}
	m.tasks[id] = handle
	m.mu.Unlock()

	go func() {
		m.updateStatus(id, TaskRunning, nil)
		err := fn(ctx, m.output)
		switch {
		case err == context.Canceled:
			m.updateStatus(id, TaskCancelled, err)
		case err == nil:
			m.updateStatus(id, TaskSucceeded, nil)
		default:
			m.updateStatus(id, TaskFailed, err)
		}
	}()

	return handle
}

func (m *TaskManager) updateStatus(id string, status TaskStatus, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	handle, ok := m.tasks[id]
	if !ok {
		return
	}
	handle.Status = status
	handle.Error = err
}

// Cancel cancels a task by ID.
func (m *TaskManager) Cancel(id string) bool {
	m.mu.Lock()
	handle, ok := m.tasks[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	handle.cancel()
	return true
}

// Tasks lists tasks.
func (m *TaskManager) Tasks() []*TaskHandle {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*TaskHandle, 0, len(m.tasks))
	for _, t := range m.tasks {
		copy := *t
		list = append(list, &copy)
	}
	return list
}

// DescribeTask returns handle by ID.
func (m *TaskManager) DescribeTask(id string) (*TaskHandle, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.tasks[id]
	if !ok {
		return nil, false
	}
	copy := *h
	return &copy, true
}
