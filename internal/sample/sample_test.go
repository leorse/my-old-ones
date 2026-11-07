package sample

import (
	"context"
	"path/filepath"
	"testing"

	"moo/internal/db"
	"moo/internal/repo"
)

func newStore(t *testing.T) *repo.Store {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "s.moo"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	return repo.NewStore(d)
}

func count(t *testing.T, s *repo.Store, q string, args ...any) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGenerateIsConsistent(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	res, err := Generate(ctx, s, Options{MaxPeople: 700})
	if err != nil {
		t.Fatal(err)
	}
	if res.People < 600 || res.People > 760 {
		t.Fatalf("personnes: %d", res.People)
	}
	if res.RootID == "" || res.Unions == 0 || res.Events < res.People {
		t.Fatalf("%+v", res)
	}
	if r, _ := s.Root(ctx); r != res.RootID {
		t.Fatal("racine non enregistrée")
	}
	if count(t, s, `SELECT COUNT(*) FROM history_entries`) != 0 {
		t.Fatal("l'historique doit être vide")
	}
	if h, _ := s.HistoryState(ctx); h.CanUndo {
		t.Fatal("annulation possible sur un projet d'exemple neuf")
	}

	// intégrité SQLite
	var ic string
	s.DB.QueryRow(`PRAGMA integrity_check`).Scan(&ic)
	if ic != "ok" {
		t.Fatal(ic)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM pragma_foreign_key_check`); n != 0 {
		t.Fatalf("%d violations de clés étrangères", n)
	}

	// cohérence chronologique (années grégoriennes)
	ev := func(typ string) string {
		return `(SELECT person_id, MIN(sort_key)/10000 AS y FROM events WHERE type='` + typ + `' AND sort_key IS NOT NULL GROUP BY person_id)`
	}
	if n := count(t, s, `SELECT COUNT(*) FROM `+ev("birth")+` b JOIN `+ev("death")+` d USING(person_id) WHERE d.y < b.y`); n != 0 {
		t.Errorf("%d décès avant la naissance", n)
	}
	// parent trop jeune (< 15 ans) ou enfant né après le décès de la mère / bien après celui du père
	q := `SELECT COUNT(*) FROM parent_child pc
		JOIN ` + ev("birth") + ` cb ON cb.person_id = pc.child_id
		JOIN ` + ev("birth") + ` pb ON pb.person_id = pc.parent_id
		WHERE pc.kind = 'biological' AND cb.y - pb.y < 15`
	if n := count(t, s, q); n != 0 {
		t.Errorf("%d parents biologiques de moins de 15 ans", n)
	}
	q = `SELECT COUNT(*) FROM parent_child pc
		JOIN ` + ev("birth") + ` cb ON cb.person_id = pc.child_id
		JOIN ` + ev("death") + ` pd ON pd.person_id = pc.parent_id
		WHERE pc.kind = 'biological' AND cb.y > pd.y + CASE pc.parent_role WHEN 'father' THEN 1 ELSE 0 END`
	if n := count(t, s, q); n != 0 {
		t.Errorf("%d enfants nés après le décès d'un parent", n)
	}
	// tous les biologiques ont un père et une mère distincts ; filiations rattachées à une union
	if n := count(t, s, `SELECT COUNT(*) FROM parent_child WHERE kind='biological' AND union_id IS NULL
		AND child_id IN (SELECT child_id FROM parent_child GROUP BY child_id HAVING COUNT(*) = 2)`); n != 0 {
		t.Errorf("%d filiations de couples sans union", n)
	}

	// diversité attendue
	if count(t, s, `SELECT COUNT(*) FROM events WHERE date_calendar='republican'`) == 0 {
		t.Error("aucune date républicaine")
	}
	if count(t, s, `SELECT COUNT(*) FROM parent_child WHERE kind='adoptive'`) == 0 {
		t.Log("aucune adoption sur cet échantillon")
	}
	if count(t, s, `SELECT COUNT(*) FROM names WHERE type='marital'`) == 0 {
		t.Error("aucun nom marital")
	}
	if count(t, s, `SELECT COUNT(*) FROM persons WHERE living='living'`) == 0 {
		t.Error("personne vivante attendue")
	}

	// la racine a une ascendance profonde et l'arbre se calcule
	td, err := s.GetTree(ctx, res.RootID, 6, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(td.Nodes) < 15 {
		t.Errorf("arbre trop petit autour de la racine: %d", len(td.Nodes))
	}
}

func TestGenerateDeterministic(t *testing.T) {
	ctx := context.Background()
	a, err := Generate(ctx, newStore(t), Options{MaxPeople: 120})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(ctx, newStore(t), Options{MaxPeople: 120})
	if err != nil {
		t.Fatal(err)
	}
	a.RootID, b.RootID = "", ""
	if a != b {
		t.Fatalf("non déterministe: %+v / %+v", a, b)
	}
}
