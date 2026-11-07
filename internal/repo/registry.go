package repo

// tracked liste les tables dont les écritures sont journalisées.
// Hors liste : project_settings, history_*, provider_cache, schema_migrations.
var tracked = map[string]bool{
	"persons": true, "names": true, "places": true, "unions": true,
	"union_partners": true, "events": true, "parent_child": true,
	"sources": true, "citations": true, "media": true, "media_links": true,
	"tags": true, "person_tags": true, "external_links": true, "gedcom_extras": true,
}

// dependent décrit une table dont les lignes doivent être supprimées avec leur
// parent. Les suppressions en cascade SQL ne sont pas journalisées : sans cette
// liste, l'annulation d'une suppression ne restaurerait pas les dépendants.
// TestDependentsCoverCascades vérifie qu'elle reste synchrone avec le schéma.
type dependent struct {
	table, column string
	// Pour les liens polymorphes (citations, media_links) :
	typeColumn, typeValue string
}

// nullifier décrit une référence ON DELETE SET NULL : à la suppression du
// parent, la colonne est remise à NULL via une mise à jour journalisée.
type nullifier struct{ table, column string }

var nullifiers = map[string][]nullifier{
	"places": {{"places", "parent_id"}, {"events", "place_id"}, {"media", "place_id"}},
	"media":  {{"persons", "profile_media_id"}},
	"unions": {{"parent_child", "union_id"}},
}

var dependents = map[string][]dependent{
	"persons": {
		{table: "names", column: "person_id"},
		{table: "gedcom_extras", column: "owner_id", typeColumn: "owner_table", typeValue: "persons"},
		{table: "events", column: "person_id"},
		{table: "parent_child", column: "child_id"},
		{table: "parent_child", column: "parent_id"},
		{table: "union_partners", column: "person_id"},
		{table: "person_tags", column: "person_id"},
		{table: "external_links", column: "person_id"},
		{table: "citations", column: "target_id", typeColumn: "target_type", typeValue: "person"},
		{table: "media_links", column: "target_id", typeColumn: "target_type", typeValue: "person"},
	},
	"unions": {
		{table: "union_partners", column: "union_id"},
		{table: "events", column: "union_id"},
		{table: "gedcom_extras", column: "owner_id", typeColumn: "owner_table", typeValue: "unions"},
		{table: "citations", column: "target_id", typeColumn: "target_type", typeValue: "union"},
	},
	"events": {
		{table: "gedcom_extras", column: "owner_id", typeColumn: "owner_table", typeValue: "events"},
		{table: "citations", column: "target_id", typeColumn: "target_type", typeValue: "event"},
		{table: "media_links", column: "target_id", typeColumn: "target_type", typeValue: "event"},
	},
	"names": {
		{table: "gedcom_extras", column: "owner_id", typeColumn: "owner_table", typeValue: "names"},
		{table: "citations", column: "target_id", typeColumn: "target_type", typeValue: "name"},
	},
	"parent_child": {
		{table: "citations", column: "target_id", typeColumn: "target_type", typeValue: "parent_child"},
	},
	"sources": {
		{table: "gedcom_extras", column: "owner_id", typeColumn: "owner_table", typeValue: "sources"},
		{table: "citations", column: "source_id"},
		{table: "media_links", column: "target_id", typeColumn: "target_type", typeValue: "source"},
	},
	"media": {
		{table: "media_links", column: "media_id"},
	},
	"tags": {
		{table: "person_tags", column: "tag_id"},
	},
}
