package repo

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"moo/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.moo"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	return NewStore(d)
}

func mustPerson(t *testing.T, s *Store, surname string) string {
	t.Helper()
	id, err := s.CreatePerson(context.Background(), NewPerson{Surname: surname, GivenNames: []string{"Jean"}, Sex: "M"})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateUndoRedo(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	mustPerson(t, s, "Dupont")

	if n, _ := s.CountPersons(ctx); n != 1 {
		t.Fatalf("attendu 1 personne, obtenu %d", n)
	}
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountPersons(ctx); n != 0 {
		t.Fatalf("après annulation: %d", n)
	}
	if _, err := s.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	ps, _ := s.ListPersons(ctx)
	if len(ps) != 1 || ps[0].Surname != "Dupont" || ps[0].GivenNames[0] != "Jean" {
		t.Fatalf("après rétablissement: %+v", ps)
	}
	if _, err := s.Redo(ctx); !errors.Is(err, ErrNothingToRedo) {
		t.Fatalf("attendu ErrNothingToRedo, obtenu %v", err)
	}
}

func TestNewMutationClearsRedo(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	mustPerson(t, s, "A")
	s.Undo(ctx)
	mustPerson(t, s, "B")
	if h, _ := s.HistoryState(ctx); h.CanRedo {
		t.Fatal("la pile rétablir aurait dû être vidée")
	}
}

func TestDeletePersonUndoRestoresDependents(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := mustPerson(t, s, "Parent")
	b := mustPerson(t, s, "Enfant")
	if _, err := s.LinkParent(ctx, b, a, "father", "biological"); err != nil {
		t.Fatal(err)
	}
	// événement de naissance, avec citation
	_, err := s.Mutate(ctx, "ev", func(t *Tx) error {
		if err := t.Insert("sources", Row{"id": "s1", "title": "Acte"}); err != nil {
			return err
		}
		if err := t.Insert("events", Row{"id": "e1", "type": "birth", "person_id": a, "y1": 1850, "date_precision": "year"}); err != nil {
			return err
		}
		return t.Insert("citations", Row{"id": "c1", "source_id": "s1", "target_type": "event", "target_id": "e1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	count := func(table string) int {
		var n int
		s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
		return n
	}
	if err := s.DeletePerson(ctx, a); err != nil {
		t.Fatal(err)
	}
	for _, tb := range []string{"events", "citations", "parent_child"} {
		if count(tb) != 0 {
			t.Fatalf("%s devrait être vide après suppression", tb)
		}
	}
	if count("names") != 1 {
		t.Fatalf("le nom de l'enfant doit subsister")
	}
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if count("persons") != 2 || count("names") != 2 || count("events") != 1 || count("citations") != 1 || count("parent_child") != 1 {
		t.Fatal("la suppression n'a pas été entièrement annulée")
	}
	// rétablir puis réannuler reste cohérent
	if _, err := s.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if count("persons") != 1 {
		t.Fatal("rétablissement incomplet")
	}
}

func TestUpdateUndo(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id := mustPerson(t, s, "X")
	if _, err := s.Mutate(ctx, "fav", func(tx *Tx) error {
		return tx.Update("persons", id, Row{"is_favorite": 1, "notes_md": "bio"})
	}); err != nil {
		t.Fatal(err)
	}
	s.Undo(ctx)
	var fav int
	var notes string
	s.DB.QueryRow(`SELECT is_favorite, notes_md FROM persons WHERE id=?`, id).Scan(&fav, &notes)
	if fav != 0 || notes != "" {
		t.Fatalf("update non annulé: %d %q", fav, notes)
	}
}

func TestMutateRollbackOnError(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.Mutate(ctx, "boom", func(tx *Tx) error {
		if err := tx.Insert("tags", Row{"id": "t1", "name": "x"}); err != nil {
			return err
		}
		return errors.New("échec")
	})
	if err == nil {
		t.Fatal("erreur attendue")
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&n)
	if n != 0 {
		t.Fatal("la transaction aurait dû être annulée")
	}
	if h, _ := s.HistoryState(ctx); h.CanUndo {
		t.Fatal("aucune entrée d'historique attendue")
	}
}

func TestHistoryLimit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.Limit = 3
	for i := 0; i < 6; i++ {
		mustPerson(t, s, "P")
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM history_entries`).Scan(&n)
	if n != 3 {
		t.Fatalf("limite d'historique: %d entrées", n)
	}
	_ = ctx
}

func TestParentChildIntegrity(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a, b, c := mustPerson(t, s, "A"), mustPerson(t, s, "B"), mustPerson(t, s, "C")

	if _, err := s.LinkParent(ctx, a, a, "", ""); !errors.Is(err, ErrSelfParent) {
		t.Fatalf("auto-référence: %v", err)
	}
	// B enfant de A, C enfant de B
	if _, err := s.LinkParent(ctx, b, a, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkParent(ctx, c, b, "", ""); err != nil {
		t.Fatal(err)
	}
	// A enfant de C => cycle
	if _, err := s.LinkParent(ctx, a, c, "", ""); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle non détecté: %v", err)
	}
	// doublon
	if _, err := s.LinkParent(ctx, b, a, "", ""); err == nil {
		t.Fatal("doublon accepté")
	}
}

func TestEventOwnerConstraint(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Mutate(context.Background(), "ev", func(tx *Tx) error {
		return tx.Insert("events", Row{"id": "e", "type": "birth"}) // ni personne ni union
	})
	if err == nil {
		t.Fatal("un événement sans rattachement doit être refusé")
	}
}

func TestUntrackedTableRejected(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Mutate(context.Background(), "x", func(tx *Tx) error {
		return tx.Insert("history_entries", Row{"id": "x"})
	})
	if err == nil {
		t.Fatal("table non suivie acceptée")
	}
}

// Garde-fou : chaque clé étrangère CASCADE / SET NULL du schéma vers une table
// suivie doit figurer dans les registres, sinon l'annulation perdrait des lignes.
func TestRegistriesCoverSchema(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.DB.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		tables = append(tables, n)
	}
	rows.Close()

	has := func(list []dependent, table, col string) bool {
		for _, d := range list {
			if d.table == table && d.column == col {
				return true
			}
		}
		return false
	}
	for _, tbl := range tables {
		fk, err := s.DB.Query(`SELECT "table", "from", on_delete FROM pragma_foreign_key_list(?)`, tbl)
		if err != nil {
			t.Fatal(err)
		}
		type ref struct{ parent, col, onDelete string }
		var refs []ref
		for fk.Next() {
			var r ref
			fk.Scan(&r.parent, &r.col, &r.onDelete)
			refs = append(refs, r)
		}
		fk.Close()
		for _, r := range refs {
			if !tracked[tbl] || !tracked[r.parent] {
				continue
			}
			switch r.onDelete {
			case "CASCADE":
				if !has(dependents[r.parent], tbl, r.col) {
					t.Errorf("dependents[%s] devrait contenir %s.%s", r.parent, tbl, r.col)
				}
			case "SET NULL":
				found := false
				for _, n := range nullifiers[r.parent] {
					if n.table == tbl && n.column == r.col {
						found = true
					}
				}
				if !found {
					t.Errorf("nullifiers[%s] devrait contenir %s.%s", r.parent, tbl, r.col)
				}
			}
		}
	}
}
