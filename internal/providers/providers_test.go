package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"moo/internal/db"
)

func fastClient() (*Client, *[]time.Duration) {
	c := NewClient(1000)
	var sleeps []time.Duration
	c.Sleep = func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	return c, &sleeps
}

func TestRetryAfterAnd429(t *testing.T) {
	var n int32
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c, sleeps := fastClient()
	b, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil || string(b) != "ok" {
		t.Fatalf("%q %v", b, err)
	}
	if len(*sleeps) != 1 || (*sleeps)[0] != 2*time.Second {
		t.Fatalf("Retry-After non respecté: %v", *sleeps)
	}
	if ua != UserAgent {
		t.Fatalf("User-Agent: %q", ua)
	}
}

func TestGivesUpAfterMaxRetries(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c, sleeps := fastClient()
	_, err := c.Get(context.Background(), srv.URL, nil)
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 503 {
		t.Fatalf("erreur: %v", err)
	}
	if n != 4 || len(*sleeps) != 3 {
		t.Fatalf("tentatives=%d pauses=%v", n, *sleeps)
	}
	if (*sleeps)[1] != 2*(*sleeps)[0] {
		t.Fatalf("backoff non exponentiel: %v", *sleeps)
	}
}

func TestNoRetryOn404(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c, _ := fastClient()
	if _, err := c.Get(context.Background(), srv.URL, nil); err == nil || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("x")) }))
	defer srv.Close()
	c := NewClient(20) // 1 requête / 50 ms
	start := time.Now()
	for i := 0; i < 4; i++ {
		if _, err := c.Get(context.Background(), srv.URL, nil); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 120*time.Millisecond {
		t.Fatalf("débit non limité: %v", d)
	}
}

func TestCacheTTL(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "c.moo"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	cache := NewCache(d)
	now := time.Now()
	cache.Now = func() time.Time { return now }
	ctx := context.Background()

	cache.Put(ctx, "p", "k", []byte("v"), time.Minute)
	if b, ok, _ := cache.Get(ctx, "p", "k"); !ok || string(b) != "v" {
		t.Fatal("cache manquant")
	}
	now = now.Add(2 * time.Minute)
	if _, ok, _ := cache.Get(ctx, "p", "k"); ok {
		t.Fatal("entrée expirée servie")
	}
}

type stub struct{ calls int }

func (s *stub) Info() Info { return Info{ID: "stub", Name: "Stub"} }
func (s *stub) Search(context.Context, Query) ([]Candidate, error) {
	s.calls++
	return []Candidate{{ProviderID: "stub"}}, nil
}
func (s *stub) Fetch(context.Context, string) (Candidate, error) { return Candidate{}, nil }
func (s *stub) DeepLink(Query) string                            { return "" }

func TestRegistryPolicy(t *testing.T) {
	r := NewRegistry()
	s := &stub{}
	r.Register(s)
	ctx := context.Background()

	if _, err := r.Search(ctx, DefaultPolicy(), "stub", Query{IsLiving: true}); !errors.Is(err, ErrLivingBlocked) {
		t.Fatalf("vivant non bloqué: %v", err)
	}
	if s.calls != 0 {
		t.Fatal("le fournisseur a été appelé pour une personne vivante")
	}
	if _, err := r.Search(ctx, DefaultPolicy(), "stub", Query{}); err != nil {
		t.Fatal(err)
	}
	r.SetEnabled("stub", false)
	if _, err := r.Search(ctx, DefaultPolicy(), "stub", Query{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("désactivation: %v", err)
	}
	if _, err := r.Search(ctx, DefaultPolicy(), "nope", Query{}); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("inconnu: %v", err)
	}
}
