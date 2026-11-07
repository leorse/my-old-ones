package api

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"moo/internal/gedcom"
	"moo/internal/jobs"
	"moo/internal/project"
)

type GedcomReport = gedcom.Report

// reports conserve les rapports d'import terminés, par identifiant de tâche.
var (
	reportsMu sync.Mutex
	reports   = map[string]*gedcom.Report{}
)

// stopJobs annule les tâches en cours et attend leur arrêt (avant de fermer la base).
func (a *API) stopJobs() {
	a.jobs.CancelAll()
	a.jobs.Wait()
}

// GedcomDialog demande un fichier GEDCOM ; renvoie "" si l'utilisateur annule.
func (a *API) GedcomDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Importer un fichier GEDCOM",
		Filters: []runtime.FileFilter{
			{DisplayName: "Fichiers GEDCOM (*.ged)", Pattern: "*.ged;*.gedcom"},
			{DisplayName: "Tous les fichiers", Pattern: "*"},
		},
	})
}

// ImportGedcomNewProject crée un nouveau projet (emplacement choisi par l'utilisateur)
// puis y importe le fichier en tâche de fond. Renvoie l'identifiant de la tâche,
// ou "" si l'utilisateur annule le choix d'emplacement.
func (a *API) ImportGedcomNewProject(gedPath string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(gedPath), filepath.Ext(gedPath))
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Enregistrer le nouveau projet",
		DefaultFilename: base + project.Extension,
		Filters:         []runtime.FileFilter{{DisplayName: "Projet Moo (*.moo)", Pattern: "*.moo"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	a.stopJobs()
	if _, err := a.projects.Create(a.context(), filepath.Clean(path)); err != nil {
		return "", err
	}
	a.emit(EventProjectChanged, nil)
	return a.startGedcomImport(gedPath)
}

// ImportGedcomHere ajoute le contenu d'un fichier GEDCOM au projet ouvert
// (jamais d'écrasement) après une sauvegarde « pre-import ».
func (a *API) ImportGedcomHere(gedPath string) (string, error) {
	p, err := a.projects.Require()
	if err != nil {
		return "", err
	}
	if _, err := p.Backup(a.context(), "pre-import"); err != nil {
		return "", fmt.Errorf("sauvegarde avant import: %w", err)
	}
	return a.startGedcomImport(gedPath)
}

func (a *API) startGedcomImport(gedPath string) (string, error) {
	p, err := a.projects.Require()
	if err != nil {
		return "", err
	}
	idCh := make(chan string, 1)
	id := a.jobs.Start("gedcom-import", func(ctx context.Context, report jobs.Reporter) error {
		rep, err := gedcom.ImportFile(ctx, p.DB, gedPath, gedcom.Options{
			Progress: func(done, total int, msg string) { report(done, total, msg) },
		})
		if err != nil {
			return err
		}
		jobID := <-idCh
		reportsMu.Lock()
		reports[jobID] = rep
		reportsMu.Unlock()
		a.emit(EventHistoryChanged, nil)
		return nil
	})
	idCh <- id
	return id, nil
}

// GedcomReport renvoie le rapport d'un import terminé.
func (a *API) GedcomReport(jobID string) (*GedcomReport, error) {
	reportsMu.Lock()
	defer reportsMu.Unlock()
	if r, ok := reports[jobID]; ok {
		return r, nil
	}
	return nil, errors.New("rapport introuvable")
}
