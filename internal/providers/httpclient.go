package providers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

// UserAgent identifie l'application auprès des services tiers (spec §6.1).
// Ne contient volontairement aucune donnée personnelle.
const UserAgent = "Moo/0.1 (My Olds One, desktop genealogy app)"

const maxBody = 8 << 20

// HTTPError est renvoyée pour les statuts non réussis après les relances.
type HTTPError struct{ Status int }

func (e *HTTPError) Error() string {
	if e.Status == http.StatusTooManyRequests {
		return "service saturé, réessayez plus tard"
	}
	return fmt.Sprintf("service indisponible (HTTP %d)", e.Status)
}

// Client est un client HTTP limité en débit, avec relances bornées.
type Client struct {
	HTTP       *http.Client
	Limiter    *rate.Limiter
	MaxRetries int           // relances après la première tentative
	BaseDelay  time.Duration // délai initial, doublé à chaque relance
	MaxDelay   time.Duration // plafond (y compris pour Retry-After)
	Sleep      func(context.Context, time.Duration) error
}

// NewClient crée un client limité à ratePerSec requêtes par seconde.
func NewClient(ratePerSec float64) *Client {
	return &Client{
		HTTP:       &http.Client{Timeout: 20 * time.Second},
		Limiter:    rate.NewLimiter(rate.Limit(ratePerSec), 1),
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		Sleep:      sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Get effectue une requête GET et renvoie le corps. Respecte le débit, les
// codes 429/5xx et l'en-tête Retry-After.
func (c *Client) Get(ctx context.Context, url string, header http.Header) ([]byte, error) {
	delay := c.BaseDelay
	for attempt := 0; ; attempt++ {
		if err := c.Limiter.Wait(ctx); err != nil {
			return nil, err
		}
		body, status, retryAfter, err := c.once(ctx, url, header)
		if err == nil && status >= 200 && status < 300 {
			return body, nil
		}
		retryable := err != nil || status == http.StatusTooManyRequests || status >= 500
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !retryable || attempt >= c.MaxRetries {
			if err != nil {
				return nil, err
			}
			return nil, &HTTPError{Status: status}
		}
		wait := delay
		if retryAfter > 0 {
			wait = retryAfter
		}
		if wait > c.MaxDelay {
			wait = c.MaxDelay
		}
		if err := c.Sleep(ctx, wait); err != nil {
			return nil, err
		}
		delay *= 2
	}
}

func (c *Client) once(ctx context.Context, url string, header http.Header) ([]byte, int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, 0, 0, err
	}
	var ra time.Duration
	if s := resp.Header.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			ra = time.Duration(n) * time.Second
		}
	}
	return body, resp.StatusCode, ra, nil
}

// Cache est le cache SQLite des réponses (table provider_cache, spec §6.8).
type Cache struct {
	DB  *sql.DB
	Now func() time.Time
}

func NewCache(db *sql.DB) *Cache { return &Cache{DB: db, Now: time.Now} }

// Get renvoie la réponse en cache si elle n'a pas expiré.
func (c *Cache) Get(ctx context.Context, provider, key string) ([]byte, bool, error) {
	var body []byte
	var fetched, ttl int64
	err := c.DB.QueryRowContext(ctx,
		`SELECT response, fetched_at, ttl_sec FROM provider_cache WHERE provider=? AND key=?`,
		provider, key).Scan(&body, &fetched, &ttl)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if c.Now().Unix() >= fetched+ttl {
		return nil, false, nil
	}
	return body, true, nil
}

func (c *Cache) Put(ctx context.Context, provider, key string, body []byte, ttl time.Duration) error {
	_, err := c.DB.ExecContext(ctx,
		`INSERT INTO provider_cache(provider, key, response, fetched_at, ttl_sec) VALUES (?,?,?,?,?)
		 ON CONFLICT(provider, key) DO UPDATE SET response=excluded.response,
		   fetched_at=excluded.fetched_at, ttl_sec=excluded.ttl_sec`,
		provider, key, body, c.Now().Unix(), int64(ttl/time.Second))
	return err
}

// Clear vide le cache (réglage « Avancé »), pour un fournisseur ou tous ("").
func (c *Cache) Clear(ctx context.Context, provider string) error {
	if provider == "" {
		_, err := c.DB.ExecContext(ctx, `DELETE FROM provider_cache`)
		return err
	}
	_, err := c.DB.ExecContext(ctx, `DELETE FROM provider_cache WHERE provider=?`, provider)
	return err
}

// CachedGet lit le cache, sinon interroge le réseau et mémorise la réponse.
func CachedGet(ctx context.Context, c *Client, cache *Cache, provider, url string, ttl time.Duration) ([]byte, error) {
	if cache != nil {
		if b, ok, err := cache.Get(ctx, provider, url); err == nil && ok {
			return b, nil
		}
	}
	b, err := c.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	if cache != nil {
		_ = cache.Put(ctx, provider, url, b, ttl)
	}
	return b, nil
}
