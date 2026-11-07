# TODO : My Olds One

Dérivée de [spec.md](spec.md). Stack effective : Go + Wails v2 + React/TS + SQLite.
`[x]` = fait (socle technique), `[ ]` = à faire. Les phases suivent la feuille de route (§14).

---

## Socle technique (fait)

- [x] Projet Wails (Go + React/TS/Vite), module Go, structure `internal/`
- [x] SQLite pur Go, WAL, clés étrangères
- [x] Migrations SQL embarquées + refus des schémas plus récents
- [x] Schéma initial (§4) : personnes, noms, lieux, unions, événements, filiations, sources, citations, médias, étiquettes, liens externes, réglages, historique, cache
- [x] Historique annuler/rétablir générique (atomique, limite 500)
- [x] Suppression avec dépendants journalisés + test de cohérence registre/schéma
- [x] Intégrité : auto-référence, doublons, cycles de filiation
- [x] Type `Date` étendue (structure, validation, clé de tri grégorienne)
- [x] Gestion de projet : créer / ouvrir / fermer, dossier `.media`, récents
- [x] Sauvegardes `VACUUM INTO` avec rotation (avant migration, à la fermeture)
- [x] Tâches de fond (progression, annulation, événements)
- [x] Interface fournisseurs, registre, garde « vivants », client HTTP (UA, débit, Retry-After, relances), cache SQLite
- [x] Façade IPC + types TS générés
- [x] Coquille frontend : accueil, titre, navigation, barre d'état, Zustand, TanStack Query, i18next fr, thème, Ctrl+Z / Ctrl+Maj+Z
- [x] Écran Personnes minimal (ajout / liste / suppression)
- [x] Journal de diagnostic local

---

## Reste du socle / outillage

- [ ] CI (tests Go + front, build Wails Windows/macOS/Linux)
- [ ] Génération automatique des types dans le workflow (`wails generate module`) + vérification en CI
- [ ] Formulaires : React Hook Form + Zod
- [ ] Association de fichier `.moo` (double-clic) et ouverture via argument de ligne de commande
- [x] Projet d'exemple « grande famille » (bouton à l'accueil + `go run ./cmd/seed`, fixture `testdata/grande-famille.moo`)
- [ ] Ouverture automatique du dernier projet
- [ ] Sauvegarde avant opérations lourdes (import, fusion, suppression de branche) : brancher `Backup`
- [ ] Export / import d'archive `.moo.zip` (base + médias)
- [ ] « Enregistrer sous » / dupliquer un projet
- [ ] Index plein texte FTS5 + maintenance (triggers ou reconstruction)
- [ ] Test de régression de migrations (projets des versions précédentes)
- [ ] Jeux de données de performance (5 k / 50 k / 200 k personnes) + mesures des seuils §11
- [ ] Logs : niveau configurable, écran de diagnostic
- [ ] Updater signé, installeurs (phase 4)
- [ ] Choix de la licence (§17.6)

---

## Phase 0 : prototypes

- [ ] Comparatif bibliothèques d'arbre (React Flow, D3, family-chart) sur 5 000 nœuds : unions multiples, mise en page, fluidité, personnalisation, accessibilité
- [x] Prototype de parseur GEDCOM sur un fichier réel
- [ ] Maquettes : arbre, fiche personne, comparaison de résultat en ligne
- [ ] Décider : nom définitif, extension, médias copiés ou référencés (§17)

---

## Phase 1 : MVP

### Domaine
- [x] Parsing de dates tolérant (« 12/3/1845 », « vers 1845 », « entre 1840 et 1845 », « 3 germinal an XII »)
- [x] Calendrier républicain ↔ grégorien (1792–1806) et calendrier julien
- [x] Formats d'affichage des dates (backend ; reste : réglage dans l'UI)
- [x] Calcul Sosa-Stradonitz dynamique depuis la personne racine
- [ ] Degré de parenté avec la racine
- [x] Âges calculés (décès, événements, parents à la naissance)
- [ ] Normalisation des noms (particules, tri ignorant la particule, réglable)
- [x] Personne racine (réglage + Sosa)

### Dépôt et commandes
- [x] CRUD complet personnes, noms multiples (nom primaire unique)
- [x] CRUD événements (tous les types §4.2) avec date + lieu + description
- [x] CRUD lieux hiérarchiques (saisie manuelle), `display_name` calculé
- [x] CRUD unions + partenaires, ordre conservé
- [x] CRUD filiations (rôle, nature, union, ordre de fratrie), changement de type
- [x] Réordonnancement des enfants (reste : unions)
- [x] Déduction `living` depuis la date de décès
- [ ] Étiquettes (tags) colorées
- [ ] Tests de cas limites : unions multiples, adoptions, familles recomposées

### Interface
- [x] Vue arbre ascendant / descendant / sablier
- [x] Zoom, déplacement, recentrage (double-clic), nombre de générations (reste : mini-carte, repli de branches)
- [x] Carte personne (nom, années, sexe), infobulle, panneau latéral (reste : photo)
- [x] Actions contextuelles : ajouter parent / conjoint / enfant / frère-sœur, ouvrir fiche (reste : supprimer depuis l'arbre)
- [ ] Ajout rapide en série (fratrie), lier à une personne existante
- [x] Fiche personne : identité, événements, famille, notes (reste : aperçu Markdown, médias, sources, historique de la fiche, édition d'un événement existant)
- [x] Enregistrement automatique des fiches
- [ ] Liste de personnes (tableau triable / filtrable)
- [x] Recherche globale Ctrl/Cmd+K (nom, prénom, lieu, notes)
- [x] Suppression : aperçu d'impact, personne seule / avec descendants exclusifs / branche, délier
- [ ] Médias de base : photo de profil, galerie simple
- [ ] Raccourcis clavier (fait : Ctrl+K, Ctrl+Z/Y, flèches / Entrée / Espace dans l'arbre ; reste : nouvelle personne, ajout de proches)

---

## Phase 2 : V1

### GEDCOM
- [x] Parseur GEDCOM 5.5.1 (import) — reste : écrivain (export) et GEDCOM 7.0 (JSON-LD/« SNOTE », `INT`/`PHRASE`, `SCHMA`)
- [x] Encodages : UTF-8, ANSEL, ISO-8859-1 / Windows-1252, UTF-16, CP437, Macintosh (détection par BOM, déclaration CHAR et contrôle de validité)
- [x] Conservation des balises inconnues (`_XXXX`) à l'import (table `gedcom_extras`) — reste : les réécrire à l'export
- [x] Rapport d'import (importé / ignoré / avertissements / corrections)
- [ ] Import de médias référencés + résolution des manquants
- [x] Mode « ajouter au projet courant » avec sauvegarde préalable (reste : détection des doublons à l'import)
- [ ] Corpus de test supplémentaire (Geneanet, Heredis, Gramps, Ancestris, FamilySearch, Filae) + tests d'aller-retour
- [ ] Glisser-déposer d'un `.ged` sur la fenêtre
- [x] Import en tâche de fond avec progression et annulation (reste : mesure sur 50 000 personnes)

### Sources, médias, qualité
- [ ] CRUD sources et citations (fiabilité, détail, citation courte, date de consultation, origine)
- [ ] Médias complets : import, SHA-256 anti-doublons, miniatures en tâche de fond, EXIF, HEIC, visionneuse (zoom, rotation, plein écran), étiquetage de zones, recadrage du profil, médias manquants
- [ ] Panneau Contrôle qualité : toutes les alertes §5.4, seuils réglables, ignorer (mémorisé), corriger
- [ ] Détection de doublons (score) + assistant de fusion champ par champ
- [ ] Déplacement de branche (changer les parents)

### Exports
- [ ] Export CSV / JSON
- [ ] Export PDF (fiche, famille, arbre), moteur à arbitrer (§17.7)
- [ ] Export image PNG / SVG de l'arbre
- [ ] Option « masquer les vivants » pour tous les exports de partage

### En ligne (P1)
- [ ] Provider API Géo / BAN (communes, INSEE, départements) + base communes en cache
- [ ] Provider Nominatim (1 req/s)
- [ ] Provider matchID (décès INSEE) : recherche individuelle
- [ ] Vérification de décès en lot : cible, tâche de fond, pause / reprise, file de revue, « accepter tous les probables », rapport
- [ ] Score de correspondance explicable (Jaro-Winkler, phonétique, dates, lieux, sexe, seuils)
- [ ] Écran de comparaison côte à côte + citation + lien externe, « pas la bonne personne » mémorisé
- [ ] Autocomplétion de lieux, géocodage en lot
- [ ] Liens profonds (Geneanet, FamilySearch, Filae, Archives, Mémoire des hommes, Léonore)
- [ ] Réglages : activation par fournisseur, option « ne pas interroger pour les vivants » (UI)
- [ ] Mode hors ligne propre, messages d'erreur compréhensibles, journal des appels
- [ ] Tests fournisseurs avec réponses enregistrées (429, délais)

---

## Phase 3 : V2

- [ ] Vues : éventail, liste Sosa, par génération, frise chronologique, carte (MapLibre / Leaflet)
- [ ] Statistiques et rapports de complétude (§5.6), carte thermique par département
- [ ] Wikidata / Commons / Wikipédia (licence enregistrée dans le média)
- [ ] Gallica : SRU + IIIF, rattachement comme source (ARK)
- [ ] GeoNames (clé personnelle), évaluation Europeana / Internet Archive / data.bnf.fr
- [ ] Livre généalogique PDF (table des matières, index des noms)
- [ ] Site HTML statique
- [ ] Recherche phonétique avancée (français) + recherches enregistrées
- [ ] Plusieurs fenêtres
- [ ] Spécificités françaises : changements de communes (validité historique), départements historiques, Alsace-Moselle, Algérie, DOM-TOM, vocabulaire des actes
- [ ] Personnalisation de l'arbre (couleurs, champs, polices, orientation), mise en évidence des lignées

---

## Phase 4 : consolidation

- [ ] Optimisation gros arbres (virtualisation, 60 i/s à 5 000 nœuds, mise en page côté Go si besoin)
- [ ] Accessibilité (contraste AA, clavier complet, ARIA, taille de police)
- [ ] Interface anglaise
- [ ] Installeurs signés + mises à jour automatiques
- [ ] Évaluation jeu INSEE local, jeux de données d'archives open data
- [ ] E2E (création, suppression, annulation, import GEDCOM, validation en ligne)

---

## Extensions hors v1 (§16)

- [ ] Synchronisation / collaboration
- [ ] Application mobile
- [ ] OCR / transcription
- [ ] Import ADN
- [ ] Suggestions de correspondances avec d'autres arbres
- [ ] Biographies narratives
- [ ] Import Gramps XML
