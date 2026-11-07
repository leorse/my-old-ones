package api

import (
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"moo/internal/project"
	"moo/internal/repo"
	"moo/internal/sample"
)

type TreeData = repo.TreeData

// GetTree renvoie le sous-graphe autour de focusID : `up` générations
// d'ancêtres, `down` de descendants (conjoints inclus).
func (a *API) GetTree(focusID string, up, down int) (TreeData, error) {
	s, err := a.store()
	if err != nil {
		return TreeData{}, err
	}
	return s.GetTree(a.context(), focusID, up, down)
}

// SampleDialog demande où enregistrer puis crée le projet d'exemple « grande famille ».
func (a *API) SampleDialog() (*ProjectInfo, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Projet d'exemple",
		DefaultFilename: "Grande famille" + project.Extension,
		Filters:         []runtime.FileFilter{{DisplayName: "Projet Moo (*.moo)", Pattern: "*.moo"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return a.CreateSampleProject(path)
}

// CreateSampleProject crée et ouvre un projet d'exemple (≈ 800 personnes, 7 générations).
func (a *API) CreateSampleProject(path string) (*ProjectInfo, error) {
	a.stopJobs()
	p, err := a.projects.Create(a.context(), filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if _, err := sample.Generate(a.context(), p.Store, sample.Options{}); err != nil {
		_ = a.projects.Close(a.context())
		return nil, err
	}
	a.emit(EventProjectChanged, nil)
	return a.info(p)
}
