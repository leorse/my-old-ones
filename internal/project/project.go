// Package project gère le cycle de vie d'un projet : fichier .moo (SQLite),
// dossier de médias frère, sauvegardes horodatées et projets récents.
package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"moo/internal/db"
	"moo/internal/repo"
)

const Extension = ".moo"

// BackupKeep : nombre de sauvegardes conservées (spec §5.1).
const BackupKeep = 10

var ErrNoProject = errors.New("aucun projet ouvert")

// Project est un projet ouvert.
type Project struct {
	Path     string
	Name     string
	MediaDir string
	DB       *sql.DB
	Store    *repo.Store
}

// Manager ouvre un seul projet à la fois.
type Manager struct {
	mu        sync.RWMutex
	configDir string
	current   *Project
}

func NewManager(configDir string) *Manager {
	return &Manager{configDir: configDir}
}

func nameFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// MediaDirFor donne le dossier de médias associé : NomProjet.media/
func MediaDirFor(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".media"
}

// BackupDirFor donne le dossier des sauvegardes : NomProjet.backups/
func BackupDirFor(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".backups"
}

// Current renvoie le projet ouvert (nil si aucun).
func (m *Manager) Current() *Project {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Require renvoie le projet ouvert ou ErrNoProject.
func (m *Manager) Require() (*Project, error) {
	if p := m.Current(); p != nil {
		return p, nil
	}
	return nil, ErrNoProject
}

// Create crée un nouveau projet (refuse d'écraser un fichier existant) et l'ouvre.
func (m *Manager) Create(ctx context.Context, path string) (*Project, error) {
	if filepath.Ext(path) == "" {
		path += Extension
	}
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%s existe déjà", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return m.open(ctx, path, true)
}

// Open ouvre un projet existant, migrant son schéma au besoin (avec
// sauvegarde préalable).
func (m *Manager) Open(ctx context.Context, path string) (*Project, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("projet introuvable: %w", err)
	}
	return m.open(ctx, path, false)
}

func (m *Manager) open(ctx context.Context, path string, create bool) (*Project, error) {
	if err := m.Close(ctx); err != nil {
		return nil, err
	}
	d, err := db.Open(path)
	if err != nil {
		return nil, err
	}
	p := &Project{
		Path: path, Name: nameFromPath(path), MediaDir: MediaDirFor(path),
		DB: d, Store: repo.NewStore(d),
	}
	if !create {
		if pending, err := db.Pending(d); err != nil {
			d.Close()
			return nil, err
		} else if pending {
			if _, err := p.Backup(ctx, "pre-migration"); err != nil {
				d.Close()
				return nil, fmt.Errorf("sauvegarde avant migration: %w", err)
			}
		}
	}
	if err := db.Migrate(d); err != nil {
		d.Close()
		return nil, err
	}
	if err := os.MkdirAll(p.MediaDir, 0o755); err != nil {
		d.Close()
		return nil, err
	}
	if create {
		_ = p.Store.SetSetting(ctx, "created_at", time.Now().UTC().Format(time.RFC3339))
	}

	m.mu.Lock()
	m.current = p
	m.mu.Unlock()
	_ = m.addRecent(path)
	return p, nil
}

// Close sauvegarde (rotation) puis ferme le projet courant, s'il y en a un.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	p := m.current
	m.current = nil
	m.mu.Unlock()
	if p == nil {
		return nil
	}
	_, backupErr := p.Backup(ctx, "close")
	_, _ = p.DB.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	if err := p.DB.Close(); err != nil {
		return err
	}
	return backupErr
}

// Backup écrit une copie cohérente (VACUUM INTO) dans NomProjet.backups/ puis
// applique la rotation. Renvoie le chemin de la copie.
func (p *Project) Backup(ctx context.Context, reason string) (string, error) {
	dir := BackupDirFor(p.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s%s", time.Now().Format("20060102-150405.000"), reason, Extension)
	dst := filepath.Join(dir, strings.ReplaceAll(name, ".000", ""))
	if _, err := os.Stat(dst); err == nil { // deux sauvegardes dans la même seconde
		dst = strings.TrimSuffix(dst, Extension) + "-b" + Extension
	}
	if _, err := p.DB.ExecContext(ctx, "VACUUM INTO '"+strings.ReplaceAll(filepath.ToSlash(dst), "'", "''")+"'"); err != nil {
		return "", err
	}
	return dst, rotateBackups(dir, BackupKeep)
}

func rotateBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), Extension) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // horodatage en tête de nom => ordre chronologique
	for len(names) > keep {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
