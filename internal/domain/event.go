package domain

// Types d'événements (spec §4.2). Les types « union » se rattachent à une
// union, tous les autres à une personne.
var personEventTypes = map[string]bool{
	"birth": true, "baptism": true, "death": true, "burial": true, "cremation": true,
	"occupation": true, "residence": true, "census": true, "military": true,
	"decoration": true, "emigration": true, "immigration": true, "naturalization": true,
	"degree": true, "religion": true, "custom": true,
}

var unionEventTypes = map[string]bool{
	"marriage": true, "pacs": true, "divorce": true, "separation": true, "custom": true,
}

func IsPersonEventType(t string) bool { return personEventTypes[t] }
func IsUnionEventType(t string) bool  { return unionEventTypes[t] }
