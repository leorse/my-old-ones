// Commande seed : génère un projet d'exemple (grande famille fictive).
//
//	go run ./cmd/seed -out testdata/grande-famille.moo [-people 800] [-seed 1789]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"moo/internal/db"
	"moo/internal/repo"
	"moo/internal/sample"
)

func main() {
	out := flag.String("out", "testdata/grande-famille.moo", "fichier .moo à créer")
	people := flag.Int("people", 800, "nombre approximatif de personnes")
	seed := flag.Int64("seed", 0, "graine (0 = défaut)")
	force := flag.Bool("force", false, "écraser le fichier existant")
	flag.Parse()

	if _, err := os.Stat(*out); err == nil {
		if !*force {
			fmt.Fprintf(os.Stderr, "%s existe déjà (utilisez -force pour l'écraser)\n", *out)
			os.Exit(1)
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(*out + suffix)
		}
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	d, err := db.Open(*out)
	if err != nil {
		fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		fatal(err)
	}
	res, err := sample.Generate(context.Background(), repo.NewStore(d), sample.Options{Seed: *seed, MaxPeople: *people})
	if err != nil {
		fatal(err)
	}
	// fusionne le journal WAL dans le fichier principal avant fermeture
	_, _ = d.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	if err := d.Close(); err != nil {
		fatal(err)
	}
	fmt.Printf("%s : %d personnes, %d unions, %d événements, %d lieux (racine %s, génération %d)\n",
		*out, res.People, res.Unions, res.Events, res.Places, res.RootID, res.RootGen)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "erreur :", err)
	os.Exit(1)
}
