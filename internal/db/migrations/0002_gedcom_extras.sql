-- Balises GEDCOM non interprétées (_XXXX, extensions d'éditeurs…), conservées
-- telles quelles pour ne rien perdre lors d'un aller-retour (spec §5.7).
CREATE TABLE gedcom_extras (
    id          TEXT PRIMARY KEY,
    owner_table TEXT NOT NULL CHECK (owner_table IN ('persons','unions','events','names','sources')),
    owner_id    TEXT NOT NULL,
    raw         TEXT NOT NULL   -- sous-arbre GEDCOM, niveau 1 pour la ligne racine
);
CREATE INDEX idx_gedcom_extras_owner ON gedcom_extras(owner_table, owner_id);

-- Notes d'import : métadonnées de chaque import GEDCOM effectué.
CREATE TABLE gedcom_imports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    imported_at TEXT NOT NULL,
    file_name   TEXT NOT NULL,
    software    TEXT NOT NULL DEFAULT '',
    encoding    TEXT NOT NULL DEFAULT '',
    report      TEXT NOT NULL   -- rapport JSON
);
