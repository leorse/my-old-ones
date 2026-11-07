package repo

import (
	"context"
	"testing"
)

func nodeIDs(td TreeData) map[string]bool {
	m := map[string]bool{}
	for _, n := range td.Nodes {
		m[n.Person.ID] = true
	}
	return m
}

func TestGetTree(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	me := mustPerson(t, s, "Moi")
	dad := mkRel(t, s, me, RelFather, "Moi", "Papa")
	mum := mkRel(t, s, me, RelMother, "Mere", "Maman")
	gf := mkRel(t, s, dad, RelFather, "Moi", "Grand-père")
	spouse := mkRel(t, s, me, RelSpouse, "Autre", "Conjoint")
	kid := mkRel(t, s, me, RelChild, "Moi", "Enfant")
	grandkid := mkRel(t, s, kid, RelChild, "Moi", "Petit-enfant")
	sibling := mkRel(t, s, me, RelSibling, "Moi", "Frère")
	s.AddEvent(ctx, EventInput{Type: "birth", PersonID: me, Date: "3 germinal an XII"})
	s.AddEvent(ctx, EventInput{Type: "death", PersonID: me, Date: "1900"})
	u, _ := s.CreateUnion(ctx, "pacs", []string{kid, mkRel(t, s, kid, RelSpouse, "X", "Y")})
	_ = u

	td, err := s.GetTree(ctx, me, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	ids := nodeIDs(td)
	// parents, conjoint, enfant (et son conjoint) — mais ni grand-père, ni petit-enfant, ni frère
	for _, want := range []string{me, dad, mum, spouse, kid} {
		if !ids[want] {
			t.Errorf("nœud manquant %s", want)
		}
	}
	for _, no := range []string{gf, grandkid, sibling} {
		if ids[no] {
			t.Errorf("nœud en trop %s", no)
		}
	}
	for _, l := range td.Links {
		if !ids[l.ChildID] || !ids[l.ParentID] {
			t.Fatalf("lien hors sous-graphe: %+v", l)
		}
	}
	for _, n := range td.Nodes {
		if n.Person.ID == me && (n.Birth != "1804" || n.Death != "1900") {
			t.Fatalf("années: %+v", n)
		}
	}
	flags := map[string]TreeNode{}
	for _, n := range td.Nodes {
		flags[n.Person.ID] = n
	}
	// me : père et mère connus ; dad : un père (hors sous-arbre à 1 génération) mais pas de mère ; kid : aucun parent biologique de rôle mère
	if f := flags[me]; !f.HasFather || !f.HasMother || !f.HasParents {
		t.Errorf("me: %+v", f)
	}
	if f := flags[dad]; !f.HasFather || f.HasMother || !f.HasParents {
		t.Errorf("dad (grand-père hors sous-arbre mais connu): %+v", f)
	}
	if f := flags[mum]; f.HasFather || f.HasMother || f.HasParents {
		t.Errorf("mum sans parents: %+v", f)
	}
	if flags[me].ChildCount != 1 || flags[dad].ChildCount != 2 || flags[mum].ChildCount != 2 || flags[kid].ChildCount != 1 {
		t.Errorf("nombre d'enfants: me=%d dad=%d mum=%d kid=%d", flags[me].ChildCount, flags[dad].ChildCount, flags[mum].ChildCount, flags[kid].ChildCount)
	}
	td, _ = s.GetTree(ctx, me, 2, 2)
	ids = nodeIDs(td)
	if !ids[gf] || !ids[grandkid] || ids[sibling] {
		t.Fatalf("2 générations: gf=%v pk=%v frère=%v", ids[gf], ids[grandkid], ids[sibling])
	}
	// descendance seule : pas d'ancêtres
	td, _ = s.GetTree(ctx, me, 0, 1)
	if nodeIDs(td)[dad] {
		t.Fatal("ancêtre inattendu")
	}
	if _, err := s.GetTree(ctx, "nope", 1, 1); err == nil {
		t.Fatal("focus inconnu accepté")
	}
}
