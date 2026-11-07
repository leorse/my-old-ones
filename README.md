# My Olds One

Application de généalogie de bureau, locale d'abord. Voir [specs/spec.md](specs/spec.md).

**Stack (adaptation de la spec)** : Go + [Wails v2](https://wails.io) (au lieu de Rust + Tauri), React + TypeScript (Vite), SQLite (`modernc.org/sqlite`, pur Go, sans CGO).

## État : socle technique + début de la Phase 1 (modèle et fiche personne)

| Brique | Emplacement | Contenu |
|---|---|---|
| Base & migrations | `internal/db` | SQLite WAL + clés étrangères, migrations SQL embarquées (`migrations/`), refus des schémas plus récents |
| Historique | `internal/repo` | `Store.Mutate` : écritures journalisées (avant/après) en une entrée atomique ; `Undo`/`Redo`, limite 500, détection de cycles |
| Projets | `internal/project` | Fichier `.moo` + dossier `.media`, sauvegardes `VACUUM INTO` avec rotation (avant migration, à la fermeture), projets récents |
| Domaine | `internal/domain` | Date généalogique étendue (structure, validation, clé de tri) |
| Tâches de fond | `internal/jobs` | Progression, annulation, événements `job:update` |
| Fournisseurs en ligne | `internal/providers` | Interface commune, registre, garde « personnes vivantes », client HTTP (User-Agent, débit, Retry-After, relances), cache SQLite. Aucun fournisseur concret. |
| Façade IPC | `internal/api` | Méthodes liées par Wails ; types TypeScript générés dans `frontend/wailsjs` |
| Frontend | `frontend/src` | Coquille (accueil, barre de titre, navigation, barre d'état), Zustand, TanStack Query, i18next (fr), thème clair/sombre, annuler/rétablir |

Phase 1 fait : dates (parsing français, calendriers républicain / julien, affichage, âges), Sosa, CRUD personnes / noms / événements / lieux / unions / filiations, relations rapides (père, mère, conjoint, enfant, fratrie) en une opération annulable, suppression avec aperçu d'impact (seule / descendants / ascendants exclusifs), recherche globale Ctrl+K, fiche personne. Reste : voir [specs/todo.md](specs/todo.md).

## Règle structurante
Le frontend n'accède jamais à la base ni au réseau : tout passe par `internal/api`.

## Ajouter une table suivie par l'historique
1. Clé primaire texte `id` dans une migration. 2. L'ajouter à `tracked` et déclarer ses dépendances (`dependents`, `nullifiers`) dans `internal/repo/registry.go` ; `TestRegistriesCoverSchema` échoue si une clé étrangère CASCADE / SET NULL est oubliée.

## Commandes
```
wails dev          # développement
wails build        # exécutable dans build/bin
go test ./...      # tests Go
cd frontend && npm test
```

## Arbre et projet d'exemple
- **Vue Arbre** (`frontend/src/features/tree`) : sablier, ascendants (pedigree, numéros Sosa si la racine est le focus) et descendants (conjoints côte à côte, enfants sous le trait d'union du couple). Zoom molette, déplacement à la souris, double-clic = recentrer, flèches = naviguer (↑ parent, ↓ enfant, ←/→ voisin), Entrée = ouvrir la fiche, Espace = recentrer. La mise en page est une fonction pure testée (`layout.ts`) ; le backend fournit le sous-graphe via `GetTree`.
- **Projet d'exemple** : bouton « Projet d'exemple » à l'accueil, ou en ligne de commande :
  `go run ./cmd/seed -out testdata/grande-famille.moo -force [-people 800] [-seed N]`.
  Famille bretonne fictive, ≈ 800 personnes sur 7 générations (1780 → années 2000), remariages, adoptions, noms maritaux, dates républicaines et approximatives. Déterministe à graine égale. Le fichier `testdata/grande-famille.moo` est ouvrable directement.

## Import GEDCOM
`internal/gedcom` : décodage (UTF-8, UTF-16, ANSEL, Windows-1252, ISO-8859-1…), parseur de lignes (CONC/CONT, niveaux, `@@`), dates GEDCOM (calendriers grégorien / julien / républicain, `ABT`, `BEF`, `AFT`, `BET…AND`, `FROM…TO`, repli en texte libre), lieux hiérarchiques (selon `HEAD.PLAC.FORM`, code INSEE, coordonnées GPS), personnes, noms, événements, familles, filiations (`PEDI`), sources, citations, notes.
- **Jamais d'écrasement** : l'import ajoute des données dans une seule transaction (tout ou rien) et n'est pas annulable avec Ctrl+Z ; le projet est sauvegardé avant (`NomProjet.backups/…-pre-import.moo`).
- **Rien n'est perdu** : les balises non interprétées sont conservées dans `gedcom_extras` ; les dates incomprises restent en texte libre ; les liens incohérents (cycle, doublon, famille inconnue) sont ignorés et signalés dans le rapport.
- Les personnes nées il y a plus de 110 ans sont marquées décédées (signalé dans le rapport).
- Accès : accueil → « Importer un GEDCOM » (nouveau projet) ou barre de titre → « Importer un GEDCOM… » (projet ouvert).
- Pas encore : fichiers médias (signalés dans le rapport), export GEDCOM, détection de doublons.
