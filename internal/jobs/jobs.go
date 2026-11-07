// Package jobs : file de tâches de fond avec progression et annulation (spec §3.3).
package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type State string

const (
	Running   State = "running"
	Done      State = "done"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

// EventName : nom de l'événement émis vers le frontend à chaque changement.
const EventName = "job:update"

// Status est l'état sérialisable d'une tâche.
type Status struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	State     State     `json:"state"`
	Done      int       `json:"done"`
	Total     int       `json:"total"`
	Message   string    `json:"message"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
}

// Reporter permet à une tâche de publier sa progression.
type Reporter func(done, total int, message string)

// Func est le corps d'une tâche ; elle doit respecter l'annulation de ctx.
type Func func(ctx context.Context, report Reporter) error

// Emitter publie un événement vers l'interface (runtime.EventsEmit côté Wails).
type Emitter func(event string, data any)

type job struct {
	status Status
	cancel context.CancelFunc
}

type Manager struct {
	mu   sync.Mutex
	emit Emitter
	jobs map[string]*job
	seq  int
	wg   sync.WaitGroup
}

func NewManager(emit Emitter) *Manager {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Manager{emit: emit, jobs: map[string]*job{}}
}

// Start lance une tâche et renvoie son identifiant.
func (m *Manager) Start(kind string, fn Func) string {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.seq++
	id := fmt.Sprintf("job-%d", m.seq)
	j := &job{status: Status{ID: id, Kind: kind, State: Running, StartedAt: time.Now()}, cancel: cancel}
	m.jobs[id] = j
	snap := j.status
	m.mu.Unlock()
	m.emit(EventName, snap)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		err := fn(ctx, func(done, total int, msg string) {
			m.update(id, func(s *Status) { s.Done, s.Total, s.Message = done, total, msg })
		})
		m.update(id, func(s *Status) {
			switch {
			case ctx.Err() != nil:
				s.State = Cancelled
			case err != nil:
				s.State, s.Error = Failed, err.Error()
			default:
				s.State = Done
			}
		})
	}()
	return id
}

func (m *Manager) update(id string, f func(*Status)) {
	m.mu.Lock()
	j := m.jobs[id]
	f(&j.status)
	snap := j.status
	m.mu.Unlock()
	m.emit(EventName, snap)
}

// Cancel demande l'arrêt d'une tâche en cours.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || j.status.State != Running {
		return false
	}
	j.cancel()
	return true
}

// List renvoie toutes les tâches connues, les plus anciennes d'abord.
func (m *Manager) List() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j.status)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].StartedAt.Before(out[k].StartedAt) })
	return out
}

// Wait bloque jusqu'à la fin de toutes les tâches (arrêt propre, tests).
func (m *Manager) Wait() { m.wg.Wait() }

// CancelAll annule tout (fermeture de l'application ou du projet).
func (m *Manager) CancelAll() {
	m.mu.Lock()
	for _, j := range m.jobs {
		if j.status.State == Running {
			j.cancel()
		}
	}
	m.mu.Unlock()
}
