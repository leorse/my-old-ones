// Package db ouvre la base SQLite d'un projet et applique les migrations.
package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pilote SQLite pur Go (sans CGO)
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open ouvre (ou crée) la base en mode WAL avec les clés étrangères activées.
func Open(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	dsn := "file:" + filepath.ToSlash(path) + "?" + q.Encode()

	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, fmt.Errorf("ouverture de %s: %w", path, err)
	}
	return d, nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		base := strings.TrimSuffix(e.Name(), ".sql")
		num, name, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("nom de migration invalide: %s", e.Name())
		}
		v, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("version de migration invalide: %s", e.Name())
		}
		b, err := fs.ReadFile(migrationsFS, "migrations/"+e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{v, name, string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// LatestVersion renvoie la version de schéma connue de cette build.
func LatestVersion() int {
	ms, _ := loadMigrations()
	if len(ms) == 0 {
		return 0
	}
	return ms[len(ms)-1].version
}

// ErrNewerSchema signale un projet créé par une version plus récente.
var ErrNewerSchema = errors.New("le projet a été créé par une version plus récente de l'application")

// CurrentVersion lit la version de schéma appliquée (0 si base vierge).
func CurrentVersion(d *sql.DB) (int, error) {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		return 0, err
	}
	var v sql.NullInt64
	if err := d.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, err
	}
	return int(v.Int64), nil
}

// Pending indique si des migrations restent à appliquer.
func Pending(d *sql.DB) (bool, error) {
	cur, err := CurrentVersion(d)
	if err != nil {
		return false, err
	}
	return cur < LatestVersion(), nil
}

// Migrate applique les migrations manquantes, chacune dans sa transaction.
func Migrate(d *sql.DB) error {
	ms, err := loadMigrations()
	if err != nil {
		return err
	}
	cur, err := CurrentVersion(d)
	if err != nil {
		return err
	}
	if len(ms) > 0 && cur > ms[len(ms)-1].version {
		return ErrNewerSchema
	}
	for _, m := range ms {
		if m.version <= cur {
			continue
		}
		if err := apply(d, m); err != nil {
			return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
		}
	}
	return nil
}

func apply(d *sql.DB, m migration) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(m.sql); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations(version, name, applied_at) VALUES (?,?,?)`,
		m.version, m.name, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}
