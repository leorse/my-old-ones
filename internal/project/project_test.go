package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"moo/internal/repo"
)

func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewManager(filepath.Join(dir, "cfg"))
	path := filepath.Join(dir, "famille")

	p, err := m.Create(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(p.Path) != Extension {
		t.Fatalf("extension: %s", p.Path)
	}
	if st, err := os.Stat(p.MediaDir); err != nil || !st.IsDir() {
		t.Fatal("dossier médias non créé")
	}
	if _, err := p.Store.CreatePerson(ctx, repo.NewPerson{Surname: "Martin"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(ctx, path); err == nil {
		t.Fatal("écrasement d'un projet existant accepté")
	}
	// Create a échoué avant de fermer ? Le projet courant doit rester utilisable.
	if m.Current() == nil {
		t.Fatal("projet courant perdu après une création refusée")
	}

	if err := m.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Require(); err != ErrNoProject {
		t.Fatalf("Require: %v", err)
	}
	backups, _ := os.ReadDir(BackupDirFor(p.Path))
	if len(backups) == 0 {
		t.Fatal("sauvegarde à la fermeture absente")
	}

	p2, err := m.Open(ctx, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(ctx)
	if n, _ := p2.Store.CountPersons(ctx); n != 1 {
		t.Fatalf("données perdues: %d", n)
	}
	if r := m.Recents(); len(r) != 1 || r[0].Name != "famille" {
		t.Fatalf("récents: %+v", r)
	}
}

func TestBackupRotation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := NewManager(dir)
	p, err := m.Create(ctx, filepath.Join(dir, "p"))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(ctx)
	for i := 0; i < BackupKeep+4; i++ {
		if _, err := p.Backup(ctx, "test"); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(BackupDirFor(p.Path))
	if len(entries) != BackupKeep {
		t.Fatalf("rotation: %d sauvegardes", len(entries))
	}
}
