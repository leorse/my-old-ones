package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestJobLifecycle(t *testing.T) {
	var mu sync.Mutex
	var events []Status
	m := NewManager(func(_ string, d any) {
		mu.Lock()
		events = append(events, d.(Status))
		mu.Unlock()
	})

	ok := m.Start("ok", func(ctx context.Context, r Reporter) error { r(1, 2, "moitié"); return nil })
	ko := m.Start("ko", func(ctx context.Context, r Reporter) error { return errors.New("boom") })
	block := make(chan struct{})
	cn := m.Start("cancel", func(ctx context.Context, r Reporter) error { close(block); <-ctx.Done(); return ctx.Err() })
	<-block
	if !m.Cancel(cn) {
		t.Fatal("annulation refusée")
	}
	m.Wait()

	got := map[string]Status{}
	for _, s := range m.List() {
		got[s.ID] = s
	}
	if got[ok].State != Done || got[ok].Done != 1 || got[ok].Total != 2 {
		t.Errorf("ok: %+v", got[ok])
	}
	if got[ko].State != Failed || got[ko].Error != "boom" {
		t.Errorf("ko: %+v", got[ko])
	}
	if got[cn].State != Cancelled {
		t.Errorf("cancel: %+v", got[cn])
	}
	if m.Cancel(ok) {
		t.Error("annulation d'une tâche terminée")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) < 6 {
		t.Errorf("événements émis: %d", len(events))
	}
}
