package repo

import (
	"context"
	"errors"
	"testing"
)

func mkRel(t *testing.T, s *Store, of string, rel Relation, surname, given string) string {
	t.Helper()
	id, err := s.CreateRelative(context.Background(), of, rel, NewPerson{Surname: surname, GivenNames: []string{given}}, RelativeOpts{})
	if err != nil {
		t.Fatalf("%s %s: %v", rel, surname, err)
	}
	return id
}

func count(s *Store, table string) int {
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
	return n
}

func TestCreateRelatives(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	me := mustPerson(t, s, "Moi")

	pere := mkRel(t, s, me, RelFather, "Moi", "Papa")
	mere := mkRel(t, s, me, RelMother, "Autre", "Maman")
	if count(s, "unions") != 1 {
		t.Fatalf("père + mère doivent être unis: %d unions", count(s, "unions"))
	}
	if _, err := s.CreateRelative(ctx, me, RelFather, NewPerson{Surname: "X"}, RelativeOpts{}); !errors.Is(err, ErrParentExists) {
		t.Fatalf("second père biologique: %v", err)
	}
	frere := mkRel(t, s, me, RelSibling, "Moi", "Frère")
	f, _ := s.GetFamily(ctx, me)
	if len(f.Parents) != 2 || f.Parents[0].Role != "father" || f.Parents[0].Person.ID != pere || f.Parents[1].Person.ID != mere {
		t.Fatalf("parents: %+v", f.Parents)
	}
	if len(f.Siblings) != 1 || f.Siblings[0].ID != frere {
		t.Fatalf("fratrie: %+v", f.Siblings)
	}

	// conjoint + enfant : l'enfant a les deux parents, rattaché à l'union
	conj := mkRel(t, s, me, RelSpouse, "Conjoint", "C")
	enfant := mkRel(t, s, me, RelChild, "Moi", "Petit")
	f, _ = s.GetFamily(ctx, me)
	if len(f.Unions) != 1 || len(f.Unions[0].Partners) != 1 || f.Unions[0].Partners[0].ID != conj {
		t.Fatalf("unions: %+v", f.Unions)
	}
	if len(f.Unions[0].Children) != 1 || f.Unions[0].Children[0].Person.ID != enfant {
		t.Fatalf("enfants de l'union: %+v", f.Unions[0].Children)
	}
	var np int
	s.DB.QueryRow(`SELECT COUNT(*) FROM parent_child WHERE child_id = ?`, enfant).Scan(&np)
	if np != 2 {
		t.Fatalf("l'enfant devrait avoir 2 parents, a %d", np)
	}

	// une seule entrée d'historique par relation : annuler retire tout
	before := count(s, "persons")
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if count(s, "persons") != before-1 {
		t.Fatal("l'annulation d'un ajout d'enfant doit retirer une personne")
	}

	// frère sans parents
	solo := mustPerson(t, s, "Solo")
	if _, err := s.CreateRelative(ctx, solo, RelSibling, NewPerson{Surname: "S"}, RelativeOpts{}); !errors.Is(err, ErrNoParents) {
		t.Fatalf("fratrie sans parents: %v", err)
	}
	if _, err := s.CreateRelative(ctx, "nope", RelChild, NewPerson{Surname: "S"}, RelativeOpts{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("personne inconnue: %v", err)
	}
}

func TestDeathEventSyncsLiving(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	p := mustPerson(t, s, "A")
	living := func() string {
		var l string
		s.DB.QueryRow(`SELECT living FROM persons WHERE id=?`, p).Scan(&l)
		return l
	}
	id, err := s.AddEvent(ctx, EventInput{Type: "death", PersonID: p, Date: "3 germinal an XII"})
	if err != nil {
		t.Fatal(err)
	}
	if living() != "deceased" {
		t.Fatalf("living = %s", living())
	}
	evs, _ := s.PersonEvents(ctx, p)
	if len(evs) != 1 || evs[0].DateDisplay != "3 germinal an XII (24 mars 1804)" {
		t.Fatalf("événements: %+v", evs)
	}
	var key int64
	s.DB.QueryRow(`SELECT sort_key FROM events WHERE id=?`, id).Scan(&key)
	if key != 18040324 {
		t.Fatalf("sort_key = %d", key)
	}
	if err := s.DeleteEvent(ctx, id); err != nil {
		t.Fatal(err)
	}
	if living() != "unknown" {
		t.Fatalf("après suppression du décès: %s", living())
	}
	// une seule annulation rétablit l'événement ET le statut
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if living() != "deceased" || count(s, "events") != 1 {
		t.Fatalf("undo: %s, %d événements", living(), count(s, "events"))
	}
}

func TestEventValidationAndOrdering(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	p := mustPerson(t, s, "A")
	if _, err := s.AddEvent(ctx, EventInput{Type: "marriage", PersonID: p}); err == nil {
		t.Error("mariage rattaché à une personne accepté")
	}
	if _, err := s.AddEvent(ctx, EventInput{Type: "birth"}); err == nil {
		t.Error("événement sans rattachement accepté")
	}
	s.AddEvent(ctx, EventInput{Type: "death", PersonID: p, Date: "1900"})
	s.AddEvent(ctx, EventInput{Type: "birth", PersonID: p, Date: "12 mars 1845"})
	s.AddEvent(ctx, EventInput{Type: "occupation", PersonID: p, Description: "laboureur"}) // sans date
	evs, _ := s.PersonEvents(ctx, p)
	if len(evs) != 3 || evs[0].Type != "birth" || evs[1].Type != "death" || evs[2].Type != "occupation" {
		t.Fatalf("ordre chronologique: %+v", evs)
	}
	if err := s.UpdateEvent(ctx, evs[0].ID, EventPatch{Date: ptr("vers 1846")}); err != nil {
		t.Fatal(err)
	}
	evs, _ = s.PersonEvents(ctx, p)
	if evs[0].DateDisplay != "vers 1846" {
		t.Fatalf("mise à jour: %q", evs[0].DateDisplay)
	}
}

func ptr[T any](v T) *T { return &v }

func TestUnionEvents(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a, b := mustPerson(t, s, "A"), mustPerson(t, s, "B")
	u, err := s.CreateUnion(ctx, "marriage", []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUnion(ctx, "marriage", []string{a, a}); err == nil {
		t.Error("union avec soi-même acceptée")
	}
	if _, err := s.AddEvent(ctx, EventInput{Type: "marriage", UnionID: u, Date: "5/6/1870"}); err != nil {
		t.Fatal(err)
	}
	f, _ := s.GetFamily(ctx, a)
	if len(f.Unions) != 1 || len(f.Unions[0].Events) != 1 || f.Unions[0].Events[0].DateDisplay != "5 juin 1870" {
		t.Fatalf("%+v", f.Unions)
	}
	// supprimer l'union emporte ses événements
	if err := s.DeleteUnion(ctx, u); err != nil {
		t.Fatal(err)
	}
	if count(s, "events") != 0 || count(s, "union_partners") != 0 {
		t.Fatal("union mal supprimée")
	}
	s.Undo(ctx)
	if count(s, "events") != 1 || count(s, "union_partners") != 2 {
		t.Fatal("annulation incomplète")
	}
}

func TestDeleteBranches(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	// grand-père G -> père P -> enfant E ; P épouse M ; E a aussi un 2e parent M2 (hors branche) -> enfant F
	g := mustPerson(t, s, "G")
	p := mkRel(t, s, g, RelChild, "P", "p")
	mkRel(t, s, p, RelSpouse, "M", "m")
	e := mkRel(t, s, p, RelChild, "E", "e") // parents P et M
	f := mkRel(t, s, e, RelChild, "F", "f") // seul parent E
	other := mustPerson(t, s, "Other")
	if _, err := s.LinkParent(ctx, f, other, "parent", "biological"); err != nil { // F a un autre parent
		t.Fatal(err)
	}

	imp, err := s.DeleteImpact(ctx, g, DeleteDescendants)
	if err != nil {
		t.Fatal(err)
	}
	// G, P (parent unique G), E ? E a pour parents P et M, M hors ensemble -> E exclu, donc F exclu
	names := map[string]bool{}
	for _, x := range imp.People {
		names[x.Surname] = true
	}
	if !names["G"] || !names["P"] || names["E"] || names["F"] || names["M"] {
		t.Fatalf("ensemble inattendu: %v", names)
	}
	if imp.People[0].ID != g {
		t.Fatal("la cible doit venir en premier")
	}
	if err := s.DeletePeople(ctx, g, DeleteDescendants); err != nil {
		t.Fatal(err)
	}
	if count(s, "persons") != 4 { // m, e, f, other
		t.Fatalf("%d personnes restantes", count(s, "persons"))
	}
	// l'union P-M n'a plus que M : elle subsiste ; aucune union vide
	if count(s, "unions") != 1 {
		t.Fatalf("%d unions", count(s, "unions"))
	}
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if count(s, "persons") != 6 {
		t.Fatalf("annulation: %d personnes", count(s, "persons"))
	}

	// branche ascendante : supprimer E avec ses ancêtres exclusifs. G n'a que P pour enfant -> exclusif seulement si P l'est ;
	// P a deux enfants (E et ... un seul E) -> ici P n'a que E comme enfant, G que P : toute la lignée sauf M (a aussi E seul) -> M aussi
	imp, _ = s.DeleteImpact(ctx, e, DeleteAncestors)
	got := map[string]bool{}
	for _, x := range imp.People {
		got[x.Surname] = true
	}
	if !got["E"] || !got["P"] || !got["M"] || !got["G"] || got["F"] {
		t.Fatalf("ascendants exclusifs: %v", got)
	}
	// F devient isolé uniquement si Other aussi est supprimé : ici F reste lié à Other
	for _, x := range imp.BecomingIsolated {
		if x.Surname == "F" {
			t.Fatal("F est encore lié à Other")
		}
	}

	if _, err := s.DeleteImpact(ctx, g, "bogus"); err != nil {
		t.Fatal(err) // mode inconnu traité comme « seule » pour l'aperçu
	}
	if err := s.DeletePeople(ctx, g, "bogus"); err == nil {
		t.Fatal("mode inconnu accepté")
	}
}

func TestDeleteLastPartnerRemovesUnion(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := mustPerson(t, s, "A")
	b := mkRel(t, s, a, RelSpouse, "B", "b")
	s.DeletePerson(ctx, a)
	if count(s, "unions") != 1 {
		t.Fatal("l'union avec un partenaire restant doit subsister")
	}
	s.DeletePerson(ctx, b)
	if count(s, "unions") != 0 || count(s, "union_partners") != 0 {
		t.Fatal("union orpheline non supprimée")
	}
	s.Undo(ctx)
	s.Undo(ctx)
	if count(s, "unions") != 1 || count(s, "union_partners") != 2 || count(s, "persons") != 2 {
		t.Fatalf("restauration: %d unions %d partenaires %d personnes", count(s, "unions"), count(s, "union_partners"), count(s, "persons"))
	}
}

func TestPlacesHierarchy(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	fr, _ := s.CreatePlace(ctx, PlaceInput{Name: "France", Type: "country"})
	dep, _ := s.CreatePlace(ctx, PlaceInput{Name: "Ille-et-Vilaine", Type: "department", ParentID: fr})
	sm, _ := s.CreatePlace(ctx, PlaceInput{Name: "Saint-Malo", ParentID: dep, CodeINSEE: "35288"})

	ps, _ := s.ListPlaces(ctx, "malo", 0)
	if len(ps) != 1 || ps[0].DisplayName != "Saint-Malo, Ille-et-Vilaine, France" || ps[0].CodeINSEE != "35288" {
		t.Fatalf("%+v", ps)
	}
	// renommer le département met à jour les descendants
	if err := s.UpdatePlace(ctx, dep, PlaceInput{Name: "Ille et Vilaine", Type: "department", ParentID: fr}); err != nil {
		t.Fatal(err)
	}
	ps, _ = s.ListPlaces(ctx, "malo", 0)
	if ps[0].DisplayName != "Saint-Malo, Ille et Vilaine, France" {
		t.Fatalf("display non recalculé: %q", ps[0].DisplayName)
	}
	// cycle interdit
	if err := s.UpdatePlace(ctx, fr, PlaceInput{Name: "France", ParentID: sm}); err == nil {
		t.Fatal("cycle de lieux accepté")
	}
	// événement lié, puis suppression du lieu
	p := mustPerson(t, s, "A")
	s.AddEvent(ctx, EventInput{Type: "birth", PersonID: p, PlaceID: sm})
	evs, _ := s.PersonEvents(ctx, p)
	if evs[0].PlaceName != "Saint-Malo, Ille et Vilaine, France" {
		t.Fatalf("lieu de l'événement: %q", evs[0].PlaceName)
	}
	if err := s.DeletePlace(ctx, dep); err != nil {
		t.Fatal(err)
	}
	ps, _ = s.ListPlaces(ctx, "malo", 0)
	if ps[0].DisplayName != "Saint-Malo" || ps[0].ParentID != "" {
		t.Fatalf("après suppression du parent: %+v", ps[0])
	}
	s.Undo(ctx)
	ps, _ = s.ListPlaces(ctx, "malo", 0)
	if ps[0].DisplayName != "Saint-Malo, Ille et Vilaine, France" {
		t.Fatalf("annulation: %q", ps[0].DisplayName)
	}
	if _, err := s.CreatePlace(ctx, PlaceInput{Name: "  "}); err == nil {
		t.Fatal("lieu sans nom accepté")
	}
	// caractères spéciaux de LIKE
	if ps, _ := s.ListPlaces(ctx, "%", 0); len(ps) != 0 {
		t.Fatalf("« %% » doit être littéral: %d résultats", len(ps))
	}
}

func TestSosaFromStore(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	me := mustPerson(t, s, "Me")
	dad := mkRel(t, s, me, RelFather, "D", "d")
	mum := mkRel(t, s, me, RelMother, "M", "m")
	gf := mkRel(t, s, dad, RelFather, "GF", "g")

	if r, _ := s.Root(ctx); r != "" {
		t.Fatal("racine inattendue")
	}
	if err := s.SetRoot(ctx, me); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRoot(ctx, "nope"); err == nil {
		t.Fatal("racine inconnue acceptée")
	}
	root, _ := s.Root(ctx)
	n, err := s.SosaNumbers(ctx, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n[me][0] != 1 || n[dad][0] != 2 || n[mum][0] != 3 || n[gf][0] != 4 {
		t.Fatalf("%v", n)
	}
	// un parent adoptif n'est pas un ancêtre Sosa
	ad := mustPerson(t, s, "Ad")
	s.LinkParent(ctx, me, ad, "father", "adoptive")
	n, _ = s.SosaNumbers(ctx, me, 0)
	if _, ok := n[ad]; ok {
		t.Fatal("adoptif numéroté")
	}
	// racine supprimée => Root vide
	s.DeletePerson(ctx, me)
	if r, _ := s.Root(ctx); r != "" {
		t.Fatal("racine supprimée toujours renvoyée")
	}
}

func TestNamesPrimaryUnique(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	p := mustPerson(t, s, "Martin")
	marital, err := s.AddName(ctx, p, NameInput{Type: "marital", Surname: "Durand", GivenNames: []string{"Léa"}})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := s.GetCore(ctx, p)
	if len(c.Names) != 2 || !c.Names[0].IsPrimary || c.Names[0].Surname != "Martin" {
		t.Fatalf("%+v", c.Names)
	}
	if err := s.DeleteName(ctx, c.Names[0].ID); err == nil {
		t.Fatal("suppression du nom principal acceptée")
	}
	if err := s.SetPrimaryName(ctx, marital); err != nil {
		t.Fatal(err)
	}
	c, _ = s.GetCore(ctx, p)
	if c.Names[0].ID != marital || c.Names[1].IsPrimary {
		t.Fatalf("%+v", c.Names)
	}
	sum, _ := s.GetSummary(ctx, p)
	if sum.Display() != "Léa DURAND" {
		t.Fatalf("affichage: %q", sum.Display())
	}
	// annuler le changement de principal rétablit l'état précédent sans violer l'unicité
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	c, _ = s.GetCore(ctx, p)
	if c.Names[0].Surname != "Martin" {
		t.Fatalf("undo: %+v", c.Names)
	}
}

func TestUpdateParentLinkAndReorder(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	parent := mustPerson(t, s, "P")
	c1 := mkRel(t, s, parent, RelChild, "C1", "a")
	c2 := mkRel(t, s, parent, RelChild, "C2", "b")
	f, _ := s.GetFamily(ctx, parent)
	if len(f.OtherChildren) != 2 || f.OtherChildren[0].Person.ID != c1 {
		t.Fatalf("%+v", f.OtherChildren)
	}
	if err := s.ReorderChildren(ctx, []string{f.OtherChildren[1].LinkID, f.OtherChildren[0].LinkID}); err != nil {
		t.Fatal(err)
	}
	f, _ = s.GetFamily(ctx, parent)
	if f.OtherChildren[0].Person.ID != c2 {
		t.Fatalf("réordonnancement: %+v", f.OtherChildren)
	}
	if err := s.UpdateParentLink(ctx, f.OtherChildren[0].LinkID, ParentLinkPatch{Kind: ptr("adoptive")}); err != nil {
		t.Fatal(err)
	}
	f, _ = s.GetFamily(ctx, parent)
	if f.OtherChildren[0].Kind != "adoptive" {
		t.Fatal("type de lien non modifié")
	}
	if err := s.UnlinkParent(ctx, f.OtherChildren[0].LinkID); err != nil {
		t.Fatal(err)
	}
	if count(s, "persons") != 3 || count(s, "parent_child") != 1 {
		t.Fatal("délier ne doit pas supprimer les personnes")
	}
}

func TestCreatePartnerMakesFreeUnion(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	me := mustPerson(t, s, "Moi")
	p := mkRel(t, s, me, RelPartner, "Autre", "Concubin")
	var typ string
	if err := s.DB.QueryRow(`SELECT type FROM unions`).Scan(&typ); err != nil || typ != "free" {
		t.Fatalf("type d'union: %q %v", typ, err)
	}
	if count(s, "union_partners") != 2 {
		t.Fatal("deux partenaires attendus")
	}
	// l'enfant d'un concubinage est rattaché à l'union libre et a deux parents
	kid := mkRel(t, s, me, RelChild, "Moi", "Petit")
	f, _ := s.GetFamily(ctx, me)
	if len(f.Unions) != 1 || f.Unions[0].Type != "free" || len(f.Unions[0].Children) != 1 || f.Unions[0].Children[0].Person.ID != kid {
		t.Fatalf("%+v", f.Unions)
	}
	_ = p
	// annulable en une fois
	s.Undo(ctx) // enfant
	s.Undo(ctx) // concubin
	if count(s, "unions") != 0 || count(s, "persons") != 1 {
		t.Fatal("annulation incomplète")
	}
}
