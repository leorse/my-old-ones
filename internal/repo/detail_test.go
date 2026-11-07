package repo

import (
	"context"
	"testing"
)

func TestPersonDetailCalculations(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	me := mustPerson(t, s, "Moi")
	dad := mkRel(t, s, me, RelFather, "Moi", "Papa")
	s.AddEvent(ctx, EventInput{Type: "birth", PersonID: dad, Date: "1/1/1920"})
	s.AddEvent(ctx, EventInput{Type: "birth", PersonID: me, Date: "15/6/1950"})
	s.AddEvent(ctx, EventInput{Type: "occupation", PersonID: me, Date: "1/1/1975", Description: "menuisier"})
	s.AddEvent(ctx, EventInput{Type: "death", PersonID: me, Date: "14/6/2020"})
	s.SetRoot(ctx, me)

	d, err := s.GetPersonDetail(ctx, me)
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsRoot || len(d.Sosa) != 1 || d.Sosa[0] != 1 {
		t.Fatalf("racine/sosa: %v %v", d.IsRoot, d.Sosa)
	}
	if d.AgeAtDeath == nil || d.AgeAtDeath.Years != 69 || d.AgeAtDeath.Approx {
		t.Fatalf("âge au décès: %+v", d.AgeAtDeath)
	}
	if len(d.Ages) != 2 {
		t.Fatalf("âges: %+v", d.Ages)
	}
	if len(d.ParentAges) != 1 || d.ParentAges[0].Years != 30 || d.ParentAges[0].EventID != dad {
		t.Fatalf("âge du père à la naissance: %+v", d.ParentAges)
	}
	if d.Core.Living != "deceased" {
		t.Fatalf("living: %s", d.Core.Living)
	}

	dd, _ := s.GetPersonDetail(ctx, dad)
	if len(dd.Sosa) != 1 || dd.Sosa[0] != 2 {
		t.Fatalf("sosa du père: %v", dd.Sosa)
	}
	list, _ := s.SosaList(ctx)
	if len(list) != 2 || list[0].Number != 1 || list[1].Number != 2 {
		t.Fatalf("liste sosa: %+v", list)
	}
	if _, err := s.GetPersonDetail(ctx, "nope"); err == nil {
		t.Fatal("personne inconnue acceptée")
	}
}

func TestSearchPersons(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a, _ := s.CreatePerson(ctx, NewPerson{Surname: "Le Gall", GivenNames: []string{"Yves", "Marie"}})
	b, _ := s.CreatePerson(ctx, NewPerson{Surname: "Martin", GivenNames: []string{"Paul"}})
	s.UpdatePerson(ctx, b, PersonPatch{NotesMD: ptr("Marin pêcheur à Cancale")})
	pl, _ := s.CreatePlace(ctx, PlaceInput{Name: "Saint-Malo"})
	s.AddEvent(ctx, EventInput{Type: "birth", PersonID: a, PlaceID: pl})

	find := func(q string) int {
		r, err := s.SearchPersons(ctx, q, 0)
		if err != nil {
			t.Fatal(err)
		}
		return len(r)
	}
	if find("gall") != 1 || find("yves marie") != 1 || find("malo") != 1 || find("cancale") != 1 {
		t.Fatal("recherche de base")
	}
	if find("gall malo") != 1 || find("gall cancale") != 0 {
		t.Fatal("tous les mots doivent correspondre")
	}
	if find("") != 0 || find("%") != 0 {
		t.Fatal("requête vide ou caractères spéciaux")
	}
}
