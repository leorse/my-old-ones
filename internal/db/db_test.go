package db

import (
	"path/filepath"
	"testing"
)

func TestMigrateIdempotentAndPragmas(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "m.moo"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for i := 0; i < 2; i++ {
		if err := Migrate(d); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ := CurrentVersion(d); v != LatestVersion() || v < 1 {
		t.Fatalf("version %d", v)
	}
	var fk int
	var mode string
	d.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	d.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	if fk != 1 || mode != "wal" {
		t.Fatalf("fk=%d mode=%s", fk, mode)
	}
}

func TestNewerSchemaRefused(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "n.moo"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	d.Exec(`INSERT INTO schema_migrations VALUES (999,'future','x')`)
	if err := Migrate(d); err != ErrNewerSchema {
		t.Fatalf("attendu ErrNewerSchema, obtenu %v", err)
	}
}
