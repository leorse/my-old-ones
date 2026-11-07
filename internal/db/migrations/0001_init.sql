-- Schéma initial (spec §4). Toutes les tables suivies par l'historique ont une
-- clé primaire texte `id` (UUID) pour permettre l'annulation générique.

CREATE TABLE persons (
    id               TEXT PRIMARY KEY,
    sex              TEXT NOT NULL DEFAULT 'unknown' CHECK (sex IN ('M','F','other','unknown')),
    living           TEXT NOT NULL DEFAULT 'unknown' CHECK (living IN ('living','deceased','unknown')),
    is_favorite      INTEGER NOT NULL DEFAULT 0 CHECK (is_favorite IN (0,1)),
    profile_media_id TEXT REFERENCES media(id) ON DELETE SET NULL,
    notes_md         TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE names (
    id          TEXT PRIMARY KEY,
    person_id   TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    type        TEXT NOT NULL DEFAULT 'birth'
                CHECK (type IN ('birth','usage','marital','religious','nickname','other')),
    surname     TEXT NOT NULL DEFAULT '',
    given_names TEXT NOT NULL DEFAULT '[]',  -- tableau JSON ordonné
    usual_given INTEGER NOT NULL DEFAULT 0,  -- index du prénom usuel
    prefix      TEXT NOT NULL DEFAULT '',
    suffix      TEXT NOT NULL DEFAULT '',
    is_primary  INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0,1))
);
CREATE INDEX idx_names_person ON names(person_id);
CREATE INDEX idx_names_surname ON names(surname COLLATE NOCASE);
CREATE UNIQUE INDEX uq_names_primary ON names(person_id) WHERE is_primary = 1;

CREATE TABLE places (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL DEFAULT 'commune',
    parent_id    TEXT REFERENCES places(id) ON DELETE SET NULL,
    latitude     REAL,
    longitude    REAL,
    code_insee   TEXT,
    external_ids TEXT NOT NULL DEFAULT '{}',
    valid_from   TEXT,
    valid_to     TEXT,
    display_name TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_places_parent ON places(parent_id);
CREATE INDEX idx_places_insee ON places(code_insee);

CREATE TABLE unions (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL DEFAULT 'marriage'
               CHECK (type IN ('marriage','pacs','free','partnership','other')),
    status     TEXT NOT NULL DEFAULT 'ongoing' CHECK (status IN ('ongoing','ended')),
    notes_md   TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE union_partners (
    id        TEXT PRIMARY KEY,
    union_id  TEXT NOT NULL REFERENCES unions(id) ON DELETE CASCADE,
    person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    position  INTEGER NOT NULL DEFAULT 0,
    UNIQUE (union_id, person_id)
);
CREATE INDEX idx_union_partners_person ON union_partners(person_id);

CREATE TABLE events (
    id            TEXT PRIMARY KEY,
    type          TEXT NOT NULL,
    person_id     TEXT REFERENCES persons(id) ON DELETE CASCADE,
    union_id      TEXT REFERENCES unions(id) ON DELETE CASCADE,
    -- date étendue (spec §4.3)
    date_precision TEXT NOT NULL DEFAULT 'none'
                   CHECK (date_precision IN ('none','exact','month','year','about','before','after','between','text')),
    date_calendar  TEXT NOT NULL DEFAULT 'gregorian' CHECK (date_calendar IN ('gregorian','republican','julian')),
    y1 INTEGER, m1 INTEGER, d1 INTEGER,
    y2 INTEGER, m2 INTEGER, d2 INTEGER,
    sort_key      INTEGER,
    original_text TEXT NOT NULL DEFAULT '',
    place_id      TEXT REFERENCES places(id) ON DELETE SET NULL,
    description   TEXT NOT NULL DEFAULT '',
    notes_md      TEXT NOT NULL DEFAULT '',
    CHECK ((person_id IS NULL) <> (union_id IS NULL))
);
CREATE INDEX idx_events_person ON events(person_id, type);
CREATE INDEX idx_events_union ON events(union_id);
CREATE INDEX idx_events_place ON events(place_id);
CREATE INDEX idx_events_sort ON events(sort_key);

CREATE TABLE parent_child (
    id          TEXT PRIMARY KEY,
    child_id    TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    parent_id   TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    parent_role TEXT NOT NULL DEFAULT 'parent' CHECK (parent_role IN ('father','mother','parent')),
    kind        TEXT NOT NULL DEFAULT 'biological'
                CHECK (kind IN ('biological','adoptive','step','foster','legal','unknown')),
    union_id    TEXT REFERENCES unions(id) ON DELETE SET NULL,
    birth_order INTEGER,
    CHECK (child_id <> parent_id),
    UNIQUE (child_id, parent_id)
);
CREATE INDEX idx_parent_child_parent ON parent_child(parent_id);

CREATE TABLE sources (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL DEFAULT 'other',
    title      TEXT NOT NULL DEFAULT '',
    author     TEXT NOT NULL DEFAULT '',
    publisher  TEXT NOT NULL DEFAULT '',
    date       TEXT NOT NULL DEFAULT '',
    repository TEXT NOT NULL DEFAULT '',
    url        TEXT NOT NULL DEFAULT '',
    notes_md   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE citations (
    id          TEXT PRIMARY KEY,
    source_id   TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('person','event','name','union','parent_child')),
    target_id   TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    reliability INTEGER NOT NULL DEFAULT 2 CHECK (reliability BETWEEN 0 AND 3),
    quote       TEXT NOT NULL DEFAULT '',
    accessed_at TEXT,
    origin      TEXT NOT NULL DEFAULT 'manual' CHECK (origin IN ('manual','gedcom','provider'))
);
CREATE INDEX idx_citations_target ON citations(target_type, target_id);
CREATE INDEX idx_citations_source ON citations(source_id);

CREATE TABLE media (
    id             TEXT PRIMARY KEY,
    kind           TEXT NOT NULL CHECK (kind IN ('image','document','audio','video')),
    file_path      TEXT NOT NULL,
    sha256         TEXT NOT NULL,
    title          TEXT NOT NULL DEFAULT '',
    caption        TEXT NOT NULL DEFAULT '',
    date           TEXT NOT NULL DEFAULT '',
    place_id       TEXT REFERENCES places(id) ON DELETE SET NULL,
    exif           TEXT,
    thumbnail_path TEXT NOT NULL DEFAULT '',
    license        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_media_sha ON media(sha256);

CREATE TABLE media_links (
    id          TEXT PRIMARY KEY,
    media_id    TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('person','event','source')),
    target_id   TEXT NOT NULL,
    zone_x REAL, zone_y REAL, zone_w REAL, zone_h REAL   -- zone normalisée (0..1)
);
CREATE INDEX idx_media_links_target ON media_links(target_type, target_id);
CREATE INDEX idx_media_links_media ON media_links(media_id);

CREATE TABLE tags (
    id    TEXT PRIMARY KEY,
    name  TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT ''
);

CREATE TABLE person_tags (
    id        TEXT PRIMARY KEY,
    person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    tag_id    TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    UNIQUE (person_id, tag_id)
);

CREATE TABLE external_links (
    id          TEXT PRIMARY KEY,
    person_id   TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    provider_id TEXT NOT NULL,
    remote_id   TEXT NOT NULL DEFAULT '',
    url         TEXT NOT NULL DEFAULT '',
    verified_at TEXT
);
CREATE INDEX idx_external_links_person ON external_links(person_id);

-- Non suivie par l'historique : réglages du projet.
CREATE TABLE project_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Journal d'opérations (annuler / rétablir), spec §3.2.
CREATE TABLE history_entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    label      TEXT NOT NULL,
    created_at TEXT NOT NULL,
    undone     INTEGER NOT NULL DEFAULT 0 CHECK (undone IN (0,1))
);

CREATE TABLE history_changes (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL REFERENCES history_entries(id) ON DELETE CASCADE,
    seq      INTEGER NOT NULL,
    tbl      TEXT NOT NULL,
    row_id   TEXT NOT NULL,
    op       TEXT NOT NULL CHECK (op IN ('insert','update','delete')),
    before   TEXT,   -- ligne JSON avant (update, delete)
    after    TEXT    -- ligne JSON après (insert, update)
);
CREATE INDEX idx_history_changes_entry ON history_changes(entry_id, seq);

-- Cache des réponses des fournisseurs en ligne (spec §6.8).
CREATE TABLE provider_cache (
    provider   TEXT NOT NULL,
    key        TEXT NOT NULL,
    response   BLOB NOT NULL,
    fetched_at INTEGER NOT NULL,   -- epoch secondes
    ttl_sec    INTEGER NOT NULL,
    PRIMARY KEY (provider, key)
);
