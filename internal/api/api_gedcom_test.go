package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tinyGed = "0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME Anne /Martin/\n1 SEX F\n1 BIRT\n2 DATE 1 JAN 1900\n0 @I2@ INDI\n1 NAME Paul /Martin/\n1 FAMC @F1@\n0 @F1@ FAM\n1 WIFE @I1@\n1 CHIL @I2@\n0 TRLR\n"

func TestGedcomImportJob(t *testing.T) {
	dir := t.TempDir()
	ged := filepath.Join(dir, "t.ged")
	os.WriteFile(ged, []byte(tinyGed), 0o644)

	a := New(filepath.Join(dir, "cfg"))
	if _, err := a.ImportGedcomHere(ged); err == nil {
		t.Fatal("import sans projet accepté")
	}
	if _, err := a.CreateProject(filepath.Join(dir, "p")); err != nil {
		t.Fatal(err)
	}
	defer a.CloseProject()
	a.CreatePerson(PersonInput{Surname: "Existante"})
	id, err := a.ImportGedcomHere(ged)
	if err != nil {
		t.Fatal(err)
	}
	a.jobs.Wait()
	var st JobStatus
	for _, j := range a.ListJobs() {
		if j.ID == id {
			st = j
		}
	}
	if st.State != "done" {
		t.Fatalf("tâche: %+v", st)
	}
	rep, err := a.GedcomReport(id)
	if err != nil || rep.Individuals != 2 || rep.ParentLinks != 1 {
		t.Fatalf("rapport: %+v %v", rep, err)
	}
	ps, _ := a.ListPersons()
	if len(ps) != 3 {
		t.Fatalf("l'import doit s'ajouter à l'existant: %d personnes", len(ps))
	}
	// sauvegarde pré-import créée
	entries, _ := os.ReadDir(filepath.Join(dir, "p.backups"))
	found := false
	for _, e := range entries {
		found = found || strings.Contains(e.Name(), "pre-import")
	}
	if !found {
		t.Errorf("sauvegarde pre-import absente: %v", entries)
	}
	// fichier illisible => tâche en échec, rien d'ajouté
	id2, _ := a.ImportGedcomHere(filepath.Join(dir, "absent.ged"))
	a.jobs.Wait()
	for _, j := range a.ListJobs() {
		if j.ID == id2 && j.State != "failed" {
			t.Errorf("fichier absent: %+v", j)
		}
	}
	if ps, _ := a.ListPersons(); len(ps) != 3 {
		t.Error("import en échec : données ajoutées")
	}
	if _, err := a.GedcomReport("nope"); err == nil {
		t.Error("rapport inconnu accepté")
	}
}
