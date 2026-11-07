// Package logging : journal local de diagnostic avec rotation simple (spec §11).
// Aucun envoi réseau.
package logging

import (
	"log/slog"
	"os"
	"path/filepath"
)

const maxSize = 5 << 20 // 5 Mio, puis rotation vers app.log.1
const filename = "app.log"

// Setup configure slog vers <dir>/app.log et renvoie une fonction de fermeture.
// En cas d'échec d'ouverture, le journal reste sur stderr.
func Setup(dir string) (closeFn func()) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return func() {}
	}
	path := filepath.Join(dir, filename)
	if st, err := os.Stat(path); err == nil && st.Size() > maxSize {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return func() {}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return func() { f.Close() }
}
