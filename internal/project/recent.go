package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const maxRecent = 10

// Recent est une entrée de la liste des projets récents.
type Recent struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	OpenedAt time.Time `json:"openedAt"`
}

func (m *Manager) recentFile() string { return filepath.Join(m.configDir, "recent.json") }

// Recents renvoie les projets récents encore présents sur le disque.
func (m *Manager) Recents() []Recent {
	out := []Recent{}
	b, err := os.ReadFile(m.recentFile())
	if err != nil {
		return out
	}
	var all []Recent
	if json.Unmarshal(b, &all) != nil {
		return out
	}
	for _, r := range all {
		if _, err := os.Stat(r.Path); err == nil {
			out = append(out, r)
		}
	}
	return out
}

func (m *Manager) addRecent(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	list := []Recent{{Path: abs, Name: nameFromPath(abs), OpenedAt: time.Now()}}
	for _, r := range m.Recents() {
		if r.Path != abs {
			list = append(list, r)
		}
	}
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	if err := os.MkdirAll(m.configDir, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(list, "", "  ")
	return os.WriteFile(m.recentFile(), b, 0o644)
}
