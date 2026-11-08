<!--
Sync Impact Report (temporaire, à retirer avant commit)
- Version change : (modèle vierge) → 1.0.0
- Principes ajoutés : I. Local d'abord ; II. Frontière IPC unique ; III. Jamais d'écrasement
  silencieux ; IV. Historique et intégrité des données ; V. Tests obligatoires ; VI. Orientation
  France et interopérabilité
- Sections ajoutées : Contraintes techniques ; Workflow de développement
- Sections retirées : aucune
- TODO différés : aucun
-->
# My Olds One Constitution

## Core Principles

### I. Local d'abord
Toutes les données vivent sur la machine de l'utilisateur, dans un fichier projet unique (`.moo`,
SQLite) et son dossier `.media`. Toute fonction cœur MUST fonctionner hors ligne. Le réseau est un
bonus : aucune fonction cœur ne MUST en dépendre. Le projet n'inclut ni chiffrement, ni comptes,
ni synchronisation cloud, ni collaboration multi-utilisateur (v1).

### II. Frontière IPC unique
Le frontend (React/TypeScript) MUST NOT accéder directement à la base ni au réseau. Tout passe par
la façade `internal/api` (méthodes liées par Wails, types générés dans `frontend/wailsjs`). Les
règles métier résident dans le backend Go (`internal/domain`, `internal/repo`), jamais dans l'UI.
Rationale : un seul point de vérité, pas de CORS, User-Agent et débit maîtrisés.

### III. Jamais d'écrasement silencieux
Toute donnée venue de l'extérieur (fournisseur en ligne, GEDCOM) MUST passer par une validation
explicite de l'utilisateur et conserver sa provenance. Les données de personnes vivantes MUST NOT
être envoyées à un fournisseur en ligne (garde « personnes vivantes »). Le scraping de sites dont
les conditions d'utilisation l'interdisent (Geneanet notamment) est interdit.

### IV. Historique et intégrité des données
Toute écriture sur une table suivie MUST passer par `Store.Mutate` (entrée d'historique atomique,
annulable/rétablissable). Toute nouvelle table suivie MUST être déclarée dans
`internal/repo/registry.go` avec ses dépendances, et `TestRegistriesCoverSchema` MUST passer. Le
schéma évolue uniquement par migrations SQL embarquées et versionnées ; une sauvegarde est faite
avant migration, et un schéma plus récent que l'application MUST être refusé.

### V. Tests obligatoires (NON-NEGOTIABLE)
Toute logique métier (dates, relations, Sosa, GEDCOM, mise en page de l'arbre) MUST être couverte
par des tests (`go test ./...`, Vitest pour le frontend). Les fonctions de mise en page restent
pures et testées. Un correctif de bug MUST ajouter un test qui échoue avant le correctif. Aucune
fusion si les tests échouent.

### VI. Orientation France et interopérabilité
Le produit cible l'état civil français : dates généalogiques étendues, calendriers républicain et
julien, numérotation Sosa-Stradonitz, communes et départements, interface en français par défaut
(i18next, anglais prévu). L'import/export GEDCOM (5.5.1 et 7.0) MUST rester possible pour ne jamais
enfermer l'utilisateur.

## Contraintes techniques

- Stack : Go + Wails v2, React + TypeScript (Vite), SQLite via `modernc.org/sqlite` (pur Go, sans
  CGO) en mode WAL avec clés étrangères activées. Un changement de stack MUST être consigné dans
  la spécification.
- État frontend : Zustand (UI) et TanStack Query (données) ; formulaires React Hook Form + Zod.
- Client HTTP sortant : centralisé dans `internal/providers` (User-Agent identifié, limitation de
  débit, respect de `Retry-After`, relances, cache SQLite).
- Les tâches longues MUST passer par `internal/jobs` (progression, annulation, événements
  `job:update`) et ne jamais bloquer l'UI.
- Cible : projets de quelques centaines à quelques dizaines de milliers de personnes ; la fluidité
  de l'arbre sur ≈ 800 personnes (projet d'exemple) MUST être préservée.

## Workflow de développement

- La spécification de référence est `specs/spec.md`, les travaux restants sont suivis dans
  `specs/todo.md`. Toute fonctionnalité suit le cycle Spec Kit : spécification, plan, tâches,
  implémentation.
- Avant de fusionner : `go test ./...` et `cd frontend && npm test` MUST passer.
- Les décisions qui s'écartent de la spécification (comme Wails/Go à la place de Tauri/Rust) MUST
  être documentées dans `README.md` ou `specs/spec.md`.
- Complexité : commencer simple (YAGNI) ; toute abstraction ou dépendance ajoutée MUST être
  justifiée par un besoin présent.

## Governance

Cette constitution prévaut sur les autres pratiques du projet. Un amendement se fait par
modification de ce fichier, avec rationale, mise à jour du rapport d'impact et, si nécessaire, un
plan de migration des artefacts Spec Kit existants. Le versionnage suit SemVer : MAJOR pour la
suppression ou la redéfinition incompatible d'un principe, MINOR pour l'ajout d'un principe ou
d'une section ou une extension substantielle, PATCH pour les clarifications. Les plans
(`/speckit-plan`) et revues MUST vérifier la conformité aux principes ; toute dérogation MUST être
justifiée par écrit dans le plan concerné.

**Version**: 1.0.0 | **Ratified**: 2026-11-08 | **Last Amended**: 2026-11-08
