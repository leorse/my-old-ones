# Spécification : application de généalogie de bureau (Tauri + React)

> Nom : **My Olds One** (« Moo »), extension `.moo`
> Version du document : 0.1 : 7 novembre 2026
> Statut : brouillon de spécification, aucune ligne de code écrite

---

## 1. Vision et périmètre

### 1.1 Objectif
Application de bureau **locale** permettant de construire, éditer et explorer une généalogie : personnes, relations, événements, lieux, sources, médias. Elle intègre une **recherche en ligne** sur des référentiels ouverts (surtout français) pour compléter et vérifier les données.

### 1.2 Principes directeurs
1. **Local d'abord** : toutes les données vivent sur la machine de l'utilisateur, dans un fichier unique. L'application est pleinement utilisable hors ligne.
2. **Le réseau est un bonus** : aucune fonction cœur ne dépend d'Internet.
3. **Jamais d'écrasement silencieux** : toute donnée importée depuis l'extérieur passe par une validation explicite de l'utilisateur.
4. **Traçabilité** : chaque fait peut être rattaché à une source ; chaque donnée importée garde sa provenance.
5. **Compatibilité** : import/export GEDCOM pour ne jamais enfermer l'utilisateur.
6. **Orientation France** : état civil français, calendrier républicain, Sosa-Stradonitz, communes et départements, référentiels français en priorité.

### 1.3 Hors périmètre (décisions actées)
- **Pas de chiffrement** de la base, pas de mot de passe, pas de gestion de comptes.
- Pas de collaboration multi-utilisateur ni de synchronisation cloud (v1).
- Pas de scraping de sites dont les conditions d'utilisation l'interdisent (Geneanet notamment).
- Pas d'application mobile (v1).
- Pas d'ADN, pas d'OCR (extensions éventuelles ultérieures).

### 1.4 Public cible
Un généalogiste amateur ou confirmé, travaillant seul sur sa généalogie, majoritairement française, de quelques centaines à quelques dizaines de milliers de personnes.

---

## 2. Stack technique

| Couche | Choix | Justification |
|---|---|---|
| Shell applicatif | **Tauri 2** | Léger, accès natif, multiplateforme (Windows, macOS, Linux) |
| Backend | **Rust** | Parsing GEDCOM, base de données, appels réseau, tâches de fond |
| Frontend | **React + TypeScript** (Vite) | Interface, état, visualisation |
| Base de données | **SQLite** (`rusqlite`) en mode WAL | Un fichier par généalogie, requêtes, intégrité |
| Migrations | Fichiers SQL versionnés (`refinery` ou équivalent) | Évolution du schéma entre versions |
| État frontend | **Zustand** (état UI) + **TanStack Query** (données via commandes Tauri) | Simplicité, cache |
| Formulaires | React Hook Form + Zod | Validation typée |
| Visualisation arbre | À évaluer (voir §7.1) : React Flow, D3, family-chart | Prototype comparatif en début de projet |
| Cartographie | **MapLibre GL** ou Leaflet + tuiles OSM | Open source |
| Client HTTP | `reqwest` (Rust) | Appels depuis le backend uniquement |
| i18n | `i18next` | Français par défaut, anglais prévu |
| Tests | `cargo test`, Vitest, Playwright (e2e) | Voir §13 |

### Règle d'architecture structurante
**Le frontend n'accède jamais directement au réseau ni à la base.** Toutes les opérations passent par des commandes Tauri typées (IPC). Avantages : pas de CORS, User-Agent maîtrisé, limitation de débit centralisée, un seul point de vérité pour les règles métier.

---

## 3. Architecture

```
┌──────────────────────── React (WebView) ────────────────────────┐
│ UI (arbre, fiches, recherche, carte)                            │
│ Zustand (UI) · TanStack Query (cache) · commandes Tauri (IPC)   │
└───────────────────────────────┬─────────────────────────────────┘
                                │ invoke / events
┌───────────────────────────────▼─────────────────────────────────┐
│                         Backend Rust                            │
│  commands/  ── façade IPC (DTO sérialisés)                      │
│  domain/    ── règles métier : relations, dates, validations    │
│  repo/      ── accès SQLite, transactions, historique (undo)    │
│  gedcom/    ── parseur/écrivain GEDCOM 5.5.1 et 7.0             │
│  providers/ ── sources en ligne (matchID, Wikidata, Gallica…)   │
│  jobs/      ── file de tâches de fond (vérification en lot)     │
│  media/     ── stockage, miniatures, métadonnées EXIF           │
│  export/    ── PDF, images, site HTML statique                  │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                  fichier projet  .moo  (SQLite)
                  + dossier médias associé
```

### 3.1 Fichier projet
- Un projet = un fichier **SQLite** (extension proposée `.moo`) + un dossier de **médias** frère (`NomProjet.media/`).
- Option d'export en archive `.moo.zip` (base + médias) pour sauvegarde/transfert.
- Plusieurs projets possibles, un seul ouvert à la fois par fenêtre. Liste des projets récents à l'accueil.

### 3.2 Historique et annulation
- Toute modification métier est une **commande** enregistrée dans une table d'historique (opération, données avant/après, horodatage).
- **Annuler / Rétablir** illimité pendant la session, persistant entre sessions (limite configurable, ex. 500 opérations).
- Les opérations composées (ex. fusion de doublons, suppression de branche) sont atomiques : une seule entrée d'historique.

### 3.3 Tâches de fond
- File de jobs côté Rust (Tokio) : import GEDCOM volumineux, vérification de décès en lot, génération de miniatures, export PDF.
- Progression émise par événements Tauri ; annulation possible.

---

## 4. Modèle de données

### 4.1 Vue d'ensemble

```
Person ──< Name
Person ──< Event >── Place
Person ──< ParentChild >── Person
Union  ──< UnionPartner >── Person
Union  ──< Event
Event / Person / Union / ParentChild ──< Citation >── Source
Person / Event / Source ──< MediaLink >── Media
Person ──< PersonTag >── Tag
Person ──< ExternalLink
```

### 4.2 Entités

**Person**
| Champ | Type | Notes |
|---|---|---|
| id | UUID | Clé stable (indépendante de tout numéro d'affichage) |
| sex | enum | M, F, autre, inconnu |
| is_living | enum | vivant, décédé, inconnu (déduit si une date de décès existe) |
| is_favorite | bool | |
| profile_media_id | FK | Photo principale |
| notes_md | texte | Biographie / notes en Markdown |
| created_at, updated_at | datetime | |

**Name** (une personne peut avoir plusieurs noms)
| Champ | Notes |
|---|---|
| person_id | |
| type | naissance, usage, marital, religieux, surnom, autre |
| surname | Nom de famille |
| given_names | Prénoms (liste ordonnée, prénom usuel marqué) |
| prefix / suffix | Titre, particule, « Jr » |
| is_primary | Nom affiché par défaut |

**Event** (naissance, décès et autres faits : un seul mécanisme)
| Champ | Notes |
|---|---|
| id | |
| type | Voir liste ci-dessous |
| person_id ou union_id | Rattachement (l'un ou l'autre) |
| date | Structure de date étendue (§4.3) |
| place_id | FK vers Place (nullable) |
| description | Texte libre (ex. intitulé d'un métier) |
| notes_md | |

Types d'événements prévus : naissance, baptême, mariage (union), PACS (union), divorce (union), séparation (union), décès, inhumation, crémation, profession, résidence, recensement, service militaire, décoration, émigration, immigration, naturalisation, diplôme, religion, événement personnalisé.

**Place** (hiérarchique, entité propre)
| Champ | Notes |
|---|---|
| id | |
| name | Nom du lieu (ex. « Saint-Malo ») |
| type | commune, ville, lieu-dit, département, région, pays, adresse, cimetière, église… |
| parent_id | Hiérarchie (commune → département → région → pays) |
| latitude, longitude | Nullable |
| code_insee | Pour les communes françaises (nullable) |
| external_ids | JSON : GeoNames, Wikidata, etc. |
| valid_from, valid_to | Pour les noms/périmètres historiques (fusions de communes) |
| display_name | Forme complète calculée : « Saint-Malo, Ille-et-Vilaine, France » |

**ParentChild** (lien filiation)
| Champ | Notes |
|---|---|
| child_id, parent_id | |
| parent_role | père, mère, parent (neutre) |
| kind | biologique, adoptif, beau-parent, accueil, légal, inconnu |
| union_id | Union des parents concernée (nullable) |
| birth_order | Ordre de fratrie |

**Union** (couple)
| Champ | Notes |
|---|---|
| id | |
| type | mariage, PACS, union libre, partenariat, autre |
| status | en cours, terminée (divorce, séparation, décès) |
| notes_md | |

Les événements de l'union (mariage, divorce) sont des `Event` rattachés à l'union. **UnionPartner** relie 2 personnes (ou plus à terme) à une union ; l'ordre est conservé.

**Source**
| Champ | Notes |
|---|---|
| id | |
| type | acte d'état civil, registre paroissial, recensement, livre, presse, site web, témoignage, document de famille, base en ligne |
| title, author, publisher, date | |
| repository | Dépôt (ex. « AD35 »), cote |
| url | |
| notes_md | |

**Citation**
| Champ | Notes |
|---|---|
| id | |
| source_id | |
| target_type, target_id | Person, Event, Name, Union, ParentChild |
| detail | Page, vue, numéro d'acte |
| reliability | 0–3 (douteux, faible, correcte, élevée) |
| quote | Transcription courte (optionnelle) |
| accessed_at | Date de consultation (pour sources en ligne) |
| origin | manuelle, import GEDCOM, fournisseur en ligne |

**Media**
| Champ | Notes |
|---|---|
| id | |
| kind | image, document (PDF), audio, vidéo |
| file_path | Relatif au dossier médias |
| sha256 | Détection de doublons |
| title, caption, date, place_id | |
| exif | JSON (nullable) |
| thumbnail_path | |

**MediaLink** : relie un média à une personne, un événement ou une source. Pour une image, optionnellement une **zone** (x, y, largeur, hauteur normalisés) pour étiqueter une personne sur une photo de groupe.

**Tag / PersonTag** : étiquettes libres colorées.

**ExternalLink** : liens persistants d'une personne vers un identifiant externe (Wikidata Q-id, URL archives, etc.), avec date de dernière vérification.

**ProjectSettings** : clé/valeur (personne racine, préférences de dates, langue des lieux, fournisseurs activés).

**History / HistoryEntry** : journal d'opérations (§3.2).

**ProviderCache** : (provider, clé de requête, réponse brute, date, durée de vie).

### 4.3 Dates : structure étendue
Les dates généalogiques sont imprécises. Une date est stockée sous forme structurée **et** texte d'origine :

| Champ | Notes |
|---|---|
| precision | exacte, mois, année, approximative (« vers »), avant, après, entre (intervalle), texte libre |
| calendar | grégorien, républicain, julien |
| y1, m1, d1 | Date principale |
| y2, m2, d2 | Borne de fin (intervalles) |
| sort_key | Entier calculé en grégorien pour tri et filtrage |
| original_text | Saisie de l'utilisateur |

Règles :
- Conversion **calendrier républicain ↔ grégorien** pour la période 1792–1806 (an II à XIV), avec affichage des deux.
- Saisie tolérante : « 12/3/1845 », « 12 mars 1845 », « vers 1845 », « avant 1850 », « entre 1840 et 1845 », « 3 germinal an XII ».
- Affichage selon préférence : « 12 mars 1845 », « 12/03/1845 », etc.

### 4.4 Numérotation
- **Sosa-Stradonitz** calculé dynamiquement à partir de la personne racine (non stocké en base).
- Les identifiants affichés (n°) sont séquentiels pour l'utilisateur ; l'UUID reste la clé réelle.

### 4.5 Intégrité
- Clés étrangères activées, suppressions en cascade maîtrisées.
- Contraintes d'unicité : un seul nom primaire par personne ; pas de lien parent-enfant en double ; pas d'auto-référence (personne parent d'elle-même) ; détection de **cycles** généalogiques (un ancêtre ne peut être son propre descendant).

---

## 5. Fonctionnalités détaillées

### 5.1 Gestion des projets
- Créer, ouvrir, enregistrer sous, dupliquer un projet.
- Projets récents ; ouverture par double-clic sur un fichier `.moo` (association de fichier).
- Sauvegarde automatique continue (SQLite) ; **sauvegardes horodatées** automatiques avant opérations lourdes (import, fusion, suppression de branche) et à la fermeture (rotation, ex. 10 copies).
- Export/import d'archive complète (base + médias).
- Projet d'exemple fourni à l'accueil.

### 5.2 Gestion de l'arbre

**Ajout de personnes**
- Depuis une personne : ajouter **père, mère, conjoint(e), enfant, frère/sœur**. Formulaire rapide (nom, prénoms, sexe, naissance) puis fiche complète en option.
- **Lier à une personne existante** (recherche intégrée) plutôt que créer un doublon.
- Création d'une personne isolée.
- Ajout rapide en série : saisie de plusieurs enfants / une fratrie en une fenêtre.

**Édition**
- Modification de toute donnée en place, enregistrement automatique.
- Changement du type de relation (ex. biologique → adoptif).
- Réordonnancement des enfants et des unions.

**Suppression**
- Supprimer une personne : aperçu de l'**impact** (relations retirées, événements, médias liés, personnes devenant isolées).
- Options : supprimer la personne seule ; supprimer **la personne et ses descendants exclusifs** ; supprimer une **branche** (ascendante ou descendante).
- Confirmation avec décompte ; opération annulable.
- Suppression d'un lien sans supprimer les personnes (délier).

**Fusion de doublons**
- Détection automatique (noms phonétiquement proches, dates et lieux voisins, mêmes parents) avec score.
- Assistant de fusion champ par champ, conservation des deux jeux de sources et des relations, historique.

**Rattachement / déplacement**
- Déplacer une branche : changer les parents d'une personne.
- Fusionner deux arbres importés (via fusion de doublons).

**Personne racine**
- Définir / changer la personne de référence (utilisée pour Sosa, parentés, vues par défaut).

### 5.3 Fiche personne
Disposition en sections, enregistrement automatique, navigation clavier.

1. **Identité** : noms (multiples), sexe, statut vivant/décédé, étiquettes, favori.
2. **Événements** : liste chronologique éditable ; naissance et décès toujours visibles en tête ; ajout d'événements typés ou personnalisés ; date + lieu + description + sources par événement.
3. **Famille** : parents, unions (avec conjoint, date, statut), enfants par union, fratrie. Raccourcis d'ajout.
4. **Notes / biographie** : Markdown avec aperçu.
5. **Médias** : galerie, ajout par glisser-déposer, photo de profil, légendes, étiquetage de personnes sur photo, documents (actes scannés en PDF/image avec visionneuse).
6. **Sources** : citations de la personne et de chaque fait.
7. **Recherche en ligne** : onglet dédié (§6).
8. **Historique** : modifications de la fiche.

Calculs affichés : âge au décès, âge à chaque événement, âge des parents à la naissance, numéro Sosa, degré de parenté avec la racine.

### 5.4 Validations et alertes de cohérence
Alertes non bloquantes (avertissement avec explication), consultables dans un panneau « Contrôle qualité » :

- Naissance après le décès ; décès avant la naissance d'un enfant (hors posthume : père jusqu'à ~9 mois).
- Parent plus jeune que l'enfant ; écart d'âge parent-enfant improbable (mère < 12 ou > 55 ans ; père < 13 ans, seuils réglables).
- Mariage avant 15 ans ou après le décès d'un conjoint.
- Âge au décès > 120 ans.
- Union avec personne identique ; parenté circulaire.
- Événements hors ordre chronologique.
- Personne sans sexe, sans naissance, sans source.
- Lieux sans coordonnées ; lieux en doublon.
- Doublons potentiels (§5.2).

Chaque alerte peut être **ignorée** (mémorisée) ou menée à la correction.

### 5.5 Recherche et filtres
- Barre de recherche globale (raccourci `Ctrl/Cmd+K`) : nom, prénom, lieu, texte de notes.
- Recherche **phonétique** adaptée au français (variantes de Soundex/Métaphone ; équivalences orthographiques courantes : ph/f, y/i, ou/u, doublement de consonnes, accents, traits d'union).
- Filtres : période, lieu, profession, étiquette, sexe, statut, présence de sources, présence de médias.
- Recherche avancée enregistrable.
- Index plein texte SQLite (FTS5).

### 5.6 Statistiques et rapports
- Totaux (personnes, familles, générations, lieux, sources, médias).
- Noms de famille les plus fréquents ; prénoms ; professions.
- Espérance de vie, âge moyen au mariage, nombre moyen d'enfants.
- Répartition géographique (carte thermique par département).
- Anniversaires du mois (naissances, mariages, décès).
- Rapports de complétude : personnes sans naissance / sans parents / sans sources ; branches « bouts de ligne » (ancêtres sans parents connus).

### 5.7 Import / export

**GEDCOM**
- Import et export **GEDCOM 5.5.1** et **7.0**.
- Prise en charge des encodages (UTF-8, ANSEL, ISO-8859-1/Windows-1252, UTF-16) avec détection.
- Compatibilité visée : **Geneanet** (exports et imports), **Heredis**, **Gramps**, **Ancestris**, **FamilySearch**, **Filae**.
- Préservation des balises inconnues (`_XXXX`) pour ne pas perdre d'information lors d'un aller-retour.
- Rapport d'import : éléments importés, ignorés, avertissements, corrections automatiques.
- Import de médias référencés (chemins relatifs/absolus) avec résolution de fichiers manquants.
- Mode « fusionner dans le projet courant » avec détection de doublons.

**Autres formats**
- Export CSV / JSON (personnes, événements, lieux).
- Export de l'archive projet complète.
- Import depuis Gramps XML (v2, optionnel).

**Exports de présentation**
- **PDF** : fiche individuelle, fiche famille, arbre ascendant/descendant, éventail, liste Sosa, livre généalogique (table des matières, index des noms).
- **Image** (PNG, SVG) de l'arbre affiché.
- **Site HTML statique** de la généalogie (navigation par personne, index) pour partage.
- Option « **masquer les personnes vivantes** » pour tous les exports de partage (remplacées par « Vivant » ou omises).

---

## 6. Recherche en ligne (référentiels)

### 6.1 Principes
1. **Tous les appels réseau partent du backend Rust**, via une interface commune de fournisseurs.
2. **Aucune écriture sans validation** : résultat → comparaison → choix champ par champ → création éventuelle d'une citation.
3. **Respect des conditions d'utilisation** : User-Agent identifiant l'application, limitation de débit, attribution.
4. **Option « ne rien envoyer pour les personnes vivantes »** activée par défaut ; blocage des requêtes vers les services externes concernant une personne marquée vivante (sauf liens de recherche manuels).
5. **Hors ligne** : cache des réponses, fonction désactivée proprement sans réseau.
6. Activation / désactivation de chaque fournisseur dans les réglages.

### 6.2 Fournisseurs

| Priorité | Fournisseur | Rôle | Accès |
|---|---|---|---|
| P1 | **API Géo / BAN** (data.gouv.fr) | Communes françaises, codes INSEE, départements | Ouvert, sans clé |
| P1 | **Nominatim (OSM)** | Géocodage mondial, lieux-dits | Ouvert, User-Agent obligatoire, 1 requête/s |
| P1 | **matchID : décès INSEE** | Vérification de décès en France depuis 1970 | API ouverte gratuite, ~1 requête/s, recherche floue |
| P2 | **Wikidata** | Personnes notables, lieux, identifiants | Action API + SPARQL, User-Agent obligatoire, quotas |
| P2 | **Wikimedia Commons / Wikipédia** | Photos libres, résumés | Ouvert (licences CC à respecter) |
| P3 | **Gallica (BnF)** | Presse, livres, annuaires, images | SRU (recherche) + IIIF (images) ; ouvert sauf abus |
| P3 | **GeoNames** | Lieux hors France | Compte gratuit, clé personnelle |
| P3 | **Europeana, Internet Archive, data.bnf.fr** | Documents / autorités | À vérifier avant intégration |
| — | **Liens profonds** | Geneanet, FamilySearch, Filae, Archives départementales/nationales, Mémoire des hommes, Léonore | Pas d'API ouverte ou accès restreint : génération d'URL de recherche pré-remplies, ouverture dans le navigateur |

**Cas particuliers vérifiés**
- **FamilySearch** : l'API est fermée au grand public (programme « Innovator » sur dossier, demandes personnelles refusées). **Non intégré** ; liens profonds uniquement.
- **Geneanet** : pas d'API développeur publique ; accès automatisé interdit par les conditions d'utilisation. **Non intégré** ; import GEDCOM manuel + liens profonds.
- **Archives départementales / nationales, Mémoire des hommes, Léonore** : pas d'API normalisée ; liens profonds, éventuellement jeux de données open data en v2.

### 6.3 Interface commune de fournisseur (spécification fonctionnelle)

Chaque fournisseur déclare :
- `id`, nom affiché, description, URL des conditions d'utilisation.
- `capabilities` : types de recherche (personne, lieu, document), données renvoyées (dates, lieux, relations, images).
- `rate_limit` : requêtes par seconde / par jour.
- `needs_key` : clé d'API requise (saisie dans les réglages).

Opérations :
- `search(query) → [Candidat]` : candidats avec score, résumé, URL source.
- `fetch(id) → Détail` : champs normalisés (nom, naissance, décès, lieux, identifiants externes).
- `deeplink(person) → URL` : lien de recherche pré-rempli (fournisseurs sans API).

Un **Candidat** contient : champs normalisés, `provider_id`, identifiant distant, URL de consultation, score de correspondance, date de récupération.

### 6.4 Score de correspondance
Calculé côté Rust, explicable (détail affiché à l'utilisateur) :
- **Nom** : normalisation (minuscules, accents, particules « de/du/le », traits d'union), distance de Jaro-Winkler + équivalence phonétique ; prise en compte des noms de naissance vs usage.
- **Prénoms** : correspondance du premier prénom, tolérance sur l'ordre et les prénoms composés.
- **Date de naissance** : exacte = fort ; ±1 an = moyen ; inversion jour/mois détectée.
- **Lieu de naissance** : même commune = fort ; même département = moyen ; via code INSEE quand disponible.
- **Sexe** : incohérence = pénalité.
- Seuils : ≥ 0,90 « probable », 0,70–0,90 « possible », < 0,70 masqué par défaut.

### 6.5 Parcours utilisateur : validation d'un résultat
1. L'utilisateur lance une recherche depuis la fiche (onglet « Recherche en ligne ») ou en lot.
2. Liste de candidats triés par score, avec fournisseur et résumé.
3. Clic → **écran de comparaison** côte à côte : colonne « Ma fiche », colonne « Candidat », statut par champ (identique, différent, nouveau, absent).
4. L'utilisateur coche les champs à importer. Les champs différents ne sont **jamais** cochés par défaut.
5. Confirmation → mise à jour de la fiche, création d'une **Citation** (fournisseur, URL, date de consultation, fiabilité), création d'un **ExternalLink** si pertinent. Annulable.
6. Option « Ce candidat n'est pas la bonne personne » : mémorisé pour ne plus être proposé.

### 6.6 Vérification de décès en lot (matchID)
- **Cible** : personnes marquées vivantes ou sans date de décès, nées après une année seuil (réglable, défaut 1900), exclues celles nées il y a plus de 120 ans.
- Pour chaque personne : requête floue (nom, prénoms, date et lieu de naissance).
- Exécution en **tâche de fond** : file respectant 1 requête/s, barre de progression, pause / annulation, reprise.
- Résultats classés dans une **file de revue** (probable / possible / aucune correspondance) ; validation individuelle ou « accepter tous les probables » avec confirmation.
- Pour chaque décès retenu : date et lieu de décès proposés, mise à jour du statut, citation automatique.
- Rapport final exportable.
- Alternative évaluée en v2 : jeu de données INSEE agrégé en local (plusieurs Go) pour recherche sans limite de débit.

### 6.7 Autres parcours en ligne
- **Lieux** : autocomplétion à la saisie (API Géo pour la France, Nominatim ailleurs) ; création d'un lieu avec hiérarchie, code INSEE et coordonnées ; géocodage en lot des lieux sans coordonnées.
- **Personnes notables** : recherche Wikidata, comparaison, import de dates/lieux, lien Wikipédia, photo Commons avec licence enregistrée dans le média.
- **Documents Gallica** : recherche par nom + lieu + période (requête SRU), aperçu des pages (IIIF), rattachement comme **Source** avec ARK et URL stable ; option de télécharger une vignette ou une page (en respectant les conditions de réutilisation).
- **Liens de recherche** : barre d'actions « Chercher sur… » générant des URL pré-remplies depuis les données de la fiche (Geneanet, FamilySearch, Filae, Archives, Mémoire des hommes, Léonore).

### 6.8 Robustesse réseau
- Délai d'attente, relance exponentielle limitée, respect de `Retry-After` et du code 429.
- Messages d'erreur compréhensibles (« Service indisponible, réessayez plus tard »).
- Cache SQLite par fournisseur (durée de vie par type de donnée : lieux longtemps, décès quelques jours).
- Journal des appels (consultable en mode diagnostic).

---

## 7. Interface utilisateur

### 7.1 Visualisations de l'arbre

| Vue | Description |
|---|---|
| **Arbre ascendant** | Ancêtres de la personne focus, horizontal ou vertical |
| **Arbre descendant** | Descendants par unions |
| **Sablier** | Ancêtres au-dessus, descendants en dessous |
| **Éventail (fan chart)** | Ancêtres en demi-cercles par génération, couleurs par branche |
| **Liste Sosa** | Tableau des ancêtres numérotés |
| **Vue par génération** | Fratries et cousinages alignés par niveau |
| **Frise chronologique** | Vies d'une famille sur un axe temporel |
| **Carte** | Lieux d'événements, trajets de migration, filtre par période |

Interactions communes :
- Zoom / déplacement (souris et pavé tactile), mini-carte, recentrage sur une personne (double-clic).
- Repliage / dépliage de branches ; nombre de générations affiché réglable.
- Carte personne compacte : photo, nom, années ; survol = infobulle ; sélection = panneau latéral de fiche.
- **Actions contextuelles** sur chaque nœud : ajouter parent / conjoint / enfant / frère-sœur, supprimer, ouvrir la fiche.
- Mise en évidence des lignées (paternelle / maternelle), des personnes sans parents, des doublons suspects.
- Personnalisation : couleurs, champs affichés, polices, orientation.
- Performance : virtualisation / rendu progressif pour plusieurs milliers de nœuds ; objectif de fluidité à 60 i/s pour 5 000 nœuds visibles.

**Décision à prendre en phase 0** : prototype comparatif de 2–3 bibliothèques sur un jeu de données de 5 000 personnes, avec critères : gestion des unions multiples, mise en page automatique, fluidité, personnalisation, accessibilité.

### 7.2 Structure de l'application

```
┌──────────────────────────────────────────────────────────────┐
│ Barre de titre : projet · annuler/rétablir · recherche Ctrl+K│
├────────┬─────────────────────────────────────────┬───────────┤
│ Navig. │          Zone centrale                  │  Panneau  │
│ latér. │  (arbre / liste / carte / fiche …)      │  latéral  │
│        │                                         │ (fiche    │
│ Arbre  │                                         │ rapide /  │
│ Perso. │                                         │ contrôle  │
│ Lieux  │                                         │ qualité)  │
│ Sources│                                         │           │
│ Médias │                                         │           │
│ Stats  │                                         │           │
│ Qualité│                                         │           │
├────────┴─────────────────────────────────────────┴───────────┤
│ Barre d'état : nombre de personnes · tâches de fond · réseau │
└──────────────────────────────────────────────────────────────┘
```

Écrans principaux : Accueil (projets récents), Arbre, Liste de personnes (tableau triable/filtrable), Fiche personne, Lieux, Sources, Médias, Statistiques, Contrôle qualité, Recherche en ligne / file de revue, Réglages.

### 7.3 Ergonomie
- **Raccourcis clavier** : nouvelle personne, ajouter parent/conjoint/enfant, recherche globale, annuler/rétablir, navigation dans l'arbre aux flèches, ouvrir la fiche (Entrée).
- Saisie rapide : tabulation logique, autocomplétion (lieux, professions, noms déjà utilisés), mémorisation des derniers choix.
- Glisser-déposer : médias sur une fiche, fichiers `.ged` sur la fenêtre pour importer.
- Plusieurs fenêtres : ouvrir une personne ou une branche dans une seconde fenêtre pour comparer.
- Thème clair / sombre / système.
- Accessibilité : contrastes AA, navigation clavier complète, rôles ARIA, tailles de police réglables.
- Interface en français ; chaînes externalisées (i18n) pour l'anglais.

---

## 8. Gestion des médias

- Formats : JPEG, PNG, WebP, HEIC (conversion), PDF, MP3/WAV, MP4 (lecture).
- Import par glisser-déposer ou sélecteur ; copie dans le dossier médias du projet (option : référence externe).
- Détection des doublons par empreinte SHA-256.
- Miniatures générées en tâche de fond (plusieurs tailles).
- Lecture des métadonnées EXIF (date, GPS) avec proposition d'événement/lieu associé.
- Visionneuse : zoom, rotation, plein écran ; pour les actes scannés, mise en regard avec la transcription.
- Recadrage de la photo de profil.
- Étiquetage de personnes sur une photo (zones rectangulaires).
- Gestion des médias manquants (fichier déplacé) avec réassociation.

---

## 9. Données et lieux : spécificités françaises

- Base de référence des **communes** (code INSEE, département, région) chargée localement depuis l'API Géo à la première utilisation, puis mise en cache.
- Gestion des **changements de communes** (fusions, rattachements) : lieu historique conservé avec période de validité et lien vers la commune actuelle.
- **Départements historiques** (Seine, Seine-et-Oise…) et anciennes provinces en option.
- Territoires : DOM-TOM, Alsace-Moselle (1871–1918 : Empire allemand), anciens départements d'Algérie.
- **Calendrier républicain** (§4.3).
- **Particules et noms composés** : « de la Fontaine », « Le Gall » ; tri alphabétique ignorant ou non la particule (réglable).
- **Femmes mariées** : nom de naissance en nom principal ; nom marital en nom secondaire.
- Vocabulaire des actes (professions anciennes, statuts : « laboureur », « journalier »…) : liste de suggestions.

---

## 10. Réglages

- Projet : personne racine, langue d'affichage des lieux, format de date, calendrier par défaut.
- Alertes : seuils de cohérence (âges), alertes actives/inactives.
- Sauvegardes : emplacement, rotation.
- Recherche en ligne : activation par fournisseur, clés d'API (GeoNames), année seuil de vérification de décès, option « ne pas interroger pour les vivants ».
- Interface : thème, taille de police, raccourcis.
- Avancé : journal de diagnostic, vider le cache de recherche.

---

## 11. Exigences non fonctionnelles

| Domaine | Exigence |
|---|---|
| **Performance** | Ouverture d'un projet de 50 000 personnes en < 2 s ; recherche globale < 100 ms ; import GEDCOM de 50 000 personnes en < 30 s ; rendu fluide de 5 000 nœuds visibles |
| **Fiabilité** | Aucune perte de données en cas de crash : transactions SQLite, mode WAL, sauvegardes automatiques avant opérations lourdes |
| **Compatibilité** | Windows 10+, macOS 12+, distributions Linux récentes (WebKitGTK) |
| **Taille** | Installeur < 30 Mo (hors médias) |
| **Hors ligne** | 100 % des fonctions cœur utilisables sans réseau |
| **Évolutivité** | Migrations de schéma versionnées, rétrocompatibilité d'ouverture des anciens projets |
| **Maintenabilité** | Séparation domaine / infrastructure ; chaque fournisseur isolé dans son module |
| **Respect des services tiers** | User-Agent explicite, limites de débit, aucun scraping |
| **Mises à jour** | Updater Tauri (signature des mises à jour), canal stable |
| **Journalisation** | Journal local de diagnostic (rotation), sans envoi automatique |

Note : le **chiffrement** et la **protection par mot de passe** sont explicitement hors périmètre (§1.3). Le masquage des personnes vivantes concerne uniquement les exports de partage et les requêtes réseau (§5.7, §6.1).

---

## 12. Structure du dépôt (proposition)

```
genealo/
├─ src-tauri/
│  ├─ src/
│  │  ├─ commands/       # façade IPC
│  │  ├─ domain/         # personne, relations, dates, validations
│  │  ├─ repo/           # SQLite, migrations, historique
│  │  ├─ gedcom/         # parseur / écrivain
│  │  ├─ providers/      # matchid, wikidata, gallica, apigeo, nominatim, deeplinks
│  │  ├─ jobs/           # tâches de fond
│  │  ├─ media/          # stockage, miniatures
│  │  └─ export/         # pdf, images, html
│  └─ migrations/        # SQL versionné
├─ src/
│  ├─ features/          # tree, person, places, sources, media, search, online, stats, quality
│  ├─ components/        # UI partagée
│  ├─ store/             # Zustand
│  ├─ ipc/               # clients typés des commandes Tauri
│  └─ i18n/
├─ tests/                # e2e Playwright, jeux de données GEDCOM
└─ docs/                 # spec, décisions d'architecture (ADR)
```

Génération de types TypeScript depuis les DTO Rust (par ex. `ts-rs` ou `specta`) pour garantir la cohérence IPC.

---

## 13. Stratégie de test

- **Unitaires Rust** : arithmétique de dates (dont républicain), calcul Sosa, détection de cycles, parentés, règles de cohérence, score de correspondance, normalisation phonétique.
- **GEDCOM** : corpus de fichiers réels et synthétiques (Geneanet, Heredis, Gramps, Ancestris) ; tests d'**aller-retour** (import → export → import) sans perte ; tests d'encodages et de balises inconnues.
- **Fournisseurs** : tests sur réponses enregistrées (mocks HTTP), tests de limitation de débit et d'erreurs (429, délais) ; quelques tests d'intégration optionnels contre les services réels, hors CI par défaut.
- **Frontend** : tests de composants (Vitest + Testing Library), tests de store.
- **E2E** (Playwright / tauri-driver) : création d'un arbre, ajout / suppression, annulation, import GEDCOM, parcours de validation d'un résultat en ligne.
- **Performance** : jeux de 5 k / 50 k / 200 k personnes générés ; mesures automatisées des seuils §11.
- **Régression de migrations** : ouverture de projets créés avec chaque version précédente.

---

## 14. Feuille de route

### Phase 0 : cadrage et prototypes (2–3 semaines)
- Choix de la bibliothèque de visualisation (prototype sur 5 000 nœuds).
- Prototype Tauri + SQLite + commande IPC typée.
- Parseur GEDCOM : preuve de concept sur un fichier réel.
- Maquettes des écrans clés (arbre, fiche, comparaison en ligne).

### Phase 1 : MVP (≈ 8–10 semaines)
- Projet SQLite, migrations, historique annuler/rétablir.
- Modèle : personnes, noms, événements, lieux (saisie manuelle), unions, filiations.
- Dates étendues (grégorien + républicain).
- Vue arbre (ascendant/descendant/sablier) avec ajout/suppression/édition contextuels.
- Fiche personne complète, photo de profil et galerie simple.
- Recherche globale, liste de personnes.
- Sauvegardes automatiques.

### Phase 2 : V1 (≈ 8–10 semaines)
- Import/export GEDCOM 5.5.1 + 7.0, rapport d'import.
- Sources et citations.
- Médias complets (documents, étiquetage, visionneuse).
- Contrôle qualité (alertes de cohérence).
- Fusion de doublons.
- Export PDF (fiche, arbre), image, CSV/JSON.
- **Fournisseurs P1** : API Géo, Nominatim (lieux), matchID (vérification de décès individuelle et en lot) avec écran de comparaison.
- Liens profonds (Geneanet, FamilySearch, Archives, etc.).

### Phase 3 : V2 (≈ 8 semaines)
- Éventail, frise, carte, statistiques.
- Wikidata / Commons / Wikipédia.
- Gallica (recherche SRU + IIIF).
- Livre généalogique PDF, site HTML statique.
- Recherche phonétique avancée, recherches enregistrées.
- Plusieurs fenêtres.

### Phase 4 : consolidation
- Optimisation (gros arbres), accessibilité, anglais, installeurs signés, mises à jour automatiques.
- Évaluation : jeu INSEE local, jeux de données d'archives open data.

---

## 15. Risques et points de vigilance

| Risque | Impact | Parade |
|---|---|---|
| Modèle de relations trop rigide (unions multiples, adoptions, familles recomposées) | Refonte du schéma | Modèle `ParentChild` + `Union` découplés ; jeux de tests de cas limites dès la phase 1 |
| GEDCOM : variations d'implémentations | Import cassé / perte de données | Corpus réel ; conservation des balises inconnues ; tests d'aller-retour |
| Performance de la vue arbre sur gros volumes | UX dégradée | Prototype Phase 0 ; virtualisation ; calcul de mise en page côté Rust si nécessaire |
| Évolution / fermeture d'API tierces | Fonction en ligne cassée | Fournisseurs isolés ; cache ; dégradation propre vers liens profonds |
| Conditions d'utilisation des services | Blocage ou contentieux | User-Agent, limites de débit, pas de scraping ; revue des CGU avant chaque intégration |
| Faux positifs de correspondance (homonymes) | Données erronées | Score explicable, aucune écriture sans validation, jamais de pré-cochage des champs divergents |
| Données personnelles de tiers vivants envoyées à des services externes | Enjeu RGPD | Option activée par défaut : aucune requête pour les vivants ; transparence sur ce qui est envoyé |
| Dérive du périmètre (collaboration, mobile, ADN) | Retard | Liste d'extensions explicitement reportées (§16) |
| Licences des contenus importés (Commons, Gallica) | Réutilisation illicite à l'export | Enregistrement de la licence et de l'auteur dans le média ; avertissement à l'export |

---

## 16. Extensions envisagées (hors v1)

- Synchronisation / collaboration (CRDT ou serveur).
- Application mobile de consultation (Tauri 2 mobile).
- OCR et transcription assistée d'actes.
- Import de résultats ADN.
- Suggestions de correspondances avec d'autres arbres.
- Génération de biographies narratives à partir des faits.
- Import Gramps XML, jeux de données d'archives open data.
- Interface anglaise et autres langues.

---

## 17. Questions ouvertes

1. **Nom et identité** de l'application, extension de fichier définitive.
2. **Bibliothèque de visualisation** : résultat du prototype de la phase 0.
3. **Médias** : copie systématique dans le dossier du projet, ou référence externe par défaut ?
4. **Volume cible** réel (influence les choix d'indexation et de mise en page).
5. **Un projet par fichier** (retenu) : confirmer qu'aucun besoin de plusieurs arbres dans une même base.
6. **Licence** du projet (propriétaire, open source) : à décider avant d'embarquer des bibliothèques tierces.
7. **Export PDF** : moteur (génération côté Rust ou impression HTML via la WebView) à arbitrer en phase 2.
8. **Seuils d'alerte** par défaut (âges parent-enfant) à valider avec des cas réels.
9. **Vérification de décès** : API matchID en direct ou jeu INSEE local, à trancher après usage réel.

---

## Annexe A : Glossaire

- **GEDCOM** : format d'échange standard de données généalogiques.
- **Sosa-Stradonitz** : numérotation des ancêtres (1 = personne de référence, 2 = père, 3 = mère, 2n = père de n, 2n+1 = mère de n).
- **ARK** : identifiant pérenne utilisé par la BnF/Gallica.
- **IIIF** : protocole d'accès standardisé aux images numérisées.
- **SRU/CQL** : protocole et langage de requête de recherche utilisés par Gallica.
- **WAL** : mode de journalisation de SQLite améliorant robustesse et concurrence.
- **IPC** : communication entre le frontend (WebView) et le backend Rust de Tauri.

## Annexe B : Sources consultées pour la section 6

- FamilySearch : conditions d'accès à l'API (programme Innovator, accès personnel refusé) : familysearch.org/innovate/app-approval-considerations
- matchID : moteur et API de recherche des décès INSEE : data.gouv.fr (réutilisations « Moteur de recherche des personnes décédées » et « API des personnes décédées »), documentation matchid.io/link-api
- Fichier INSEE des personnes décédées : data.gouv.fr/datasets/agregation-des-fichiers-des-personnes-decedees
- Gallica : api.gouv.fr/les-api/api_gallica_IIIF ; api.bnf.fr ; data.gouv.fr/dataservices/api-gallica-iiif
- Wikimedia : politique User-Agent et robots (mediawiki.org, meta.wikimedia.org)
- Geneanet : absence d'API développeur publique (discussion Gramps) ; conditions d'utilisation interdisant l'accès automatisé

*Les fournisseurs marqués « à vérifier » (Europeana, Internet Archive, data.bnf.fr, GeoNames) et les seuils chiffrés (limites de débit, tailles) devront être revalidés au moment de l'intégration.*