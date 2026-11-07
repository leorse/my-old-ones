package api

import (
	"path/filepath"
	"testing"
)

func TestProjectAndUndoThroughAPI(t *testing.T) {
	dir := t.TempDir()
	a := New(filepath.Join(dir, "cfg"))
	if _, err := a.ListPersons(); err == nil {
		t.Fatal("ListPersons sans projet doit échouer")
	}
	info, err := a.CreateProject(filepath.Join(dir, "p"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Persons != 0 {
		t.Fatal("projet neuf non vide")
	}
	if _, err := a.CreatePerson(PersonInput{Surname: "Durand", GivenNames: []string{"Léa"}, Sex: "F"}); err != nil {
		t.Fatal(err)
	}
	ps, _ := a.ListPersons()
	if len(ps) != 1 {
		t.Fatalf("%d personnes", len(ps))
	}
	if label, err := a.Undo(); err != nil || label == "" {
		t.Fatalf("undo: %q %v", label, err)
	}
	if label, err := a.Undo(); err != nil || label != "" {
		t.Fatalf("undo à vide doit être sans effet: %q %v", label, err)
	}
	if err := a.CloseProject(); err != nil {
		t.Fatal(err)
	}
	if p, _ := a.CurrentProject(); p != nil {
		t.Fatal("projet encore ouvert")
	}
}
