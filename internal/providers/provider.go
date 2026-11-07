// Package providers : interface commune des sources en ligne (spec §6.3),
// registre, politique de confidentialité et client HTTP respectueux.
//
// Aucun fournisseur concret n'est livré avec le socle ; chacun vivra dans son
// propre sous-paquet (apigeo, nominatim, matchid, ...).
package providers

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type SearchKind string

const (
	KindPerson   SearchKind = "person"
	KindPlace    SearchKind = "place"
	KindDocument SearchKind = "document"
)

// Info décrit un fournisseur (métadonnées de la spec §6.3).
type Info struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	TermsURL    string       `json:"termsUrl"`
	Kinds       []SearchKind `json:"kinds"`
	RatePerSec  float64      `json:"ratePerSec"`
	NeedsKey    bool         `json:"needsKey"`
	// Priority : P1..P3 de la spec ; 0 = lien profond seul.
	Priority int `json:"priority"`
}

// Query est une recherche normalisée, construite depuis une fiche.
type Query struct {
	Kind       SearchKind `json:"kind"`
	Surname    string     `json:"surname"`
	GivenNames []string   `json:"givenNames"`
	Sex        string     `json:"sex"`
	BirthYear  int        `json:"birthYear"`
	BirthPlace string     `json:"birthPlace"`
	Text       string     `json:"text"`
	IsLiving   bool       `json:"isLiving"` // pour la garde de confidentialité
}

// Candidate est un résultat de recherche à valider par l'utilisateur (§6.3).
type Candidate struct {
	ProviderID string            `json:"providerId"`
	RemoteID   string            `json:"remoteId"`
	URL        string            `json:"url"`
	Summary    string            `json:"summary"`
	Fields     map[string]string `json:"fields"` // champs normalisés
	Score      float64           `json:"score"`
	FetchedAt  time.Time         `json:"fetchedAt"`
}

// Provider est l'interface que chaque source en ligne implémente.
type Provider interface {
	Info() Info
	Search(ctx context.Context, q Query) ([]Candidate, error)
	Fetch(ctx context.Context, remoteID string) (Candidate, error)
	// DeepLink renvoie une URL de recherche pré-remplie ("" si non applicable).
	DeepLink(q Query) string
}

var ErrUnknownProvider = errors.New("fournisseur inconnu")
var ErrDisabled = errors.New("fournisseur désactivé")

// Registry référence les fournisseurs et leur activation.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	enabled   map[string]bool
}

func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}, enabled: map[string]bool{}}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := p.Info().ID
	r.providers[id] = p
	r.enabled[id] = true
}

func (r *Registry) SetEnabled(id string, on bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.providers[id]; !ok {
		return ErrUnknownProvider
	}
	r.enabled[id] = on
	return nil
}

// Get renvoie un fournisseur actif.
func (r *Registry) Get(id string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	if !ok {
		return nil, ErrUnknownProvider
	}
	if !r.enabled[id] {
		return nil, ErrDisabled
	}
	return p, nil
}

// Entry associe les métadonnées d'un fournisseur à son état d'activation.
type Entry struct {
	Info    Info `json:"info"`
	Enabled bool `json:"enabled"`
}

func (r *Registry) List() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Entry, 0, len(r.providers))
	for id, p := range r.providers {
		out = append(out, Entry{Info: p.Info(), Enabled: r.enabled[id]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Info.ID < out[j].Info.ID })
	return out
}

// Policy applique les règles de confidentialité de la spec §6.1 (4).
type Policy struct {
	// BlockLiving : aucune requête externe pour une personne vivante (défaut : true).
	BlockLiving bool
}

func DefaultPolicy() Policy { return Policy{BlockLiving: true} }

var ErrLivingBlocked = errors.New("requête bloquée : la personne est marquée vivante")

// Search interroge un fournisseur après contrôle de la politique et activation.
func (r *Registry) Search(ctx context.Context, pol Policy, providerID string, q Query) ([]Candidate, error) {
	if pol.BlockLiving && q.IsLiving {
		return nil, ErrLivingBlocked
	}
	p, err := r.Get(providerID)
	if err != nil {
		return nil, err
	}
	return p.Search(ctx, q)
}
