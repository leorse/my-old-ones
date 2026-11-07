// Package api est la façade IPC exposée au frontend par Wails (spec §2,
// « règle d'architecture structurante ») : le frontend n'accède jamais
// directement à la base ni au réseau. Toutes les méthodes publiques de API
// deviennent des fonctions TypeScript typées (frontend/wailsjs).
package api

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"moo/internal/db"
	"moo/internal/jobs"
	"moo/internal/project"
	"moo/internal/providers"
	"moo/internal/repo"
)

// Version de l'application.
const Version = "0.1.0"

// Événements émis vers le frontend.
const (
	EventProjectChanged = "project:changed"
	EventHistoryChanged = "history:changed"
)

type API struct {
	ctx       context.Context
	projects  *project.Manager
	jobs      *jobs.Manager
	providers *providers.Registry
	policy    providers.Policy
}

// New construit l'API ; configDir sert aux projets récents.
func New(configDir string) *API {
	a := &API{
		projects:  project.NewManager(configDir),
		providers: providers.NewRegistry(),
		policy:    providers.DefaultPolicy(),
	}
	a.jobs = jobs.NewManager(a.emit)
	return a
}

// Startup est appelée par Wails au démarrage.
func (a *API) Startup(ctx context.Context) { a.ctx = ctx }

// Shutdown est appelée par Wails à la fermeture : arrête les tâches et ferme
// proprement le projet (sauvegarde de fermeture incluse).
func (a *API) Shutdown(ctx context.Context) {
	a.jobs.CancelAll()
	a.jobs.Wait()
	if err := a.projects.Close(ctx); err != nil {
		slog.Error("fermeture du projet", "err", err)
	}
}

// emit publie un événement ; sans contexte Wails (tests), c'est un no-op.
func (a *API) emit(event string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, event, data)
	}
}

// --- DTO -------------------------------------------------------------------

type AppInfo struct {
	Version       string `json:"version"`
	SchemaVersion int    `json:"schemaVersion"`
}

type ProjectInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	MediaDir string `json:"mediaDir"`
	Persons  int    `json:"persons"`
}

type RecentProject struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type PersonInput struct {
	Surname    string   `json:"surname"`
	GivenNames []string `json:"givenNames"`
	Sex        string   `json:"sex"`
}

type PersonSummary = repo.PersonSummary
type HistoryState = repo.HistoryState
type JobStatus = jobs.Status
type ProviderEntry = providers.Entry

// --- Application -----------------------------------------------------------

func (a *API) AppInfo() AppInfo {
	return AppInfo{Version: Version, SchemaVersion: db.LatestVersion()}
}

// --- Projets ---------------------------------------------------------------

func (a *API) RecentProjects() []RecentProject {
	out := []RecentProject{}
	for _, r := range a.projects.Recents() {
		out = append(out, RecentProject{Path: r.Path, Name: r.Name})
	}
	return out
}

// CurrentProject renvoie le projet ouvert, ou nil.
func (a *API) CurrentProject() (*ProjectInfo, error) {
	p := a.projects.Current()
	if p == nil {
		return nil, nil
	}
	return a.info(p)
}

func (a *API) info(p *project.Project) (*ProjectInfo, error) {
	n, err := p.Store.CountPersons(a.context())
	if err != nil {
		return nil, err
	}
	return &ProjectInfo{Path: p.Path, Name: p.Name, MediaDir: p.MediaDir, Persons: n}, nil
}

func (a *API) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// NewProjectDialog demande l'emplacement via une boîte « Enregistrer sous »
// puis crée et ouvre le projet. Renvoie nil si l'utilisateur annule.
func (a *API) NewProjectDialog() (*ProjectInfo, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Nouveau projet",
		DefaultFilename: "Ma généalogie" + project.Extension,
		Filters:         []runtime.FileFilter{{DisplayName: "Projet Moo (*.moo)", Pattern: "*.moo"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return a.CreateProject(path)
}

// OpenProjectDialog demande un fichier .moo puis l'ouvre.
func (a *API) OpenProjectDialog() (*ProjectInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Ouvrir un projet",
		Filters: []runtime.FileFilter{{DisplayName: "Projet Moo (*.moo)", Pattern: "*.moo"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return a.OpenProject(path)
}

func (a *API) CreateProject(path string) (*ProjectInfo, error) {
	a.stopJobs()
	p, err := a.projects.Create(a.context(), filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	a.emit(EventProjectChanged, nil)
	return a.info(p)
}

func (a *API) OpenProject(path string) (*ProjectInfo, error) {
	a.stopJobs()
	p, err := a.projects.Open(a.context(), filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	a.emit(EventProjectChanged, nil)
	return a.info(p)
}

func (a *API) CloseProject() error {
	a.stopJobs()
	err := a.projects.Close(a.context())
	a.emit(EventProjectChanged, nil)
	return err
}

// --- Personnes et historique ----------------------------------------------

func (a *API) ListPersons() ([]PersonSummary, error) {
	p, err := a.projects.Require()
	if err != nil {
		return nil, err
	}
	return p.Store.ListPersons(a.context())
}

func (a *API) CreatePerson(in PersonInput) (string, error) {
	p, err := a.projects.Require()
	if err != nil {
		return "", err
	}
	id, err := p.Store.CreatePerson(a.context(), repo.NewPerson{
		Surname: in.Surname, GivenNames: in.GivenNames, Sex: in.Sex,
	})
	if err == nil {
		a.emit(EventHistoryChanged, nil)
	}
	return id, err
}

func (a *API) DeletePerson(id string) error {
	p, err := a.projects.Require()
	if err != nil {
		return err
	}
	err = p.Store.DeletePerson(a.context(), id)
	if err == nil {
		a.emit(EventHistoryChanged, nil)
	}
	return err
}

func (a *API) HistoryState() (HistoryState, error) {
	p, err := a.projects.Require()
	if err != nil {
		return HistoryState{}, err
	}
	return p.Store.HistoryState(a.context())
}

// Undo annule la dernière opération et renvoie son libellé.
func (a *API) Undo() (string, error) {
	p, err := a.projects.Require()
	if err != nil {
		return "", err
	}
	label, err := p.Store.Undo(a.context())
	if err == nil {
		a.emit(EventHistoryChanged, nil)
	}
	if errors.Is(err, repo.ErrNothingToUndo) {
		return "", nil
	}
	return label, err
}

func (a *API) Redo() (string, error) {
	p, err := a.projects.Require()
	if err != nil {
		return "", err
	}
	label, err := p.Store.Redo(a.context())
	if err == nil {
		a.emit(EventHistoryChanged, nil)
	}
	if errors.Is(err, repo.ErrNothingToRedo) {
		return "", nil
	}
	return label, err
}

// --- Tâches de fond et fournisseurs ---------------------------------------

func (a *API) ListJobs() []JobStatus { return a.jobs.List() }

func (a *API) CancelJob(id string) bool { return a.jobs.Cancel(id) }

func (a *API) ListProviders() []ProviderEntry { return a.providers.List() }

func (a *API) SetProviderEnabled(id string, enabled bool) error {
	return a.providers.SetEnabled(id, enabled)
}
