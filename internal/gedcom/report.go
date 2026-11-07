package gedcom

import "fmt"

// Report résume un import (spec §5.7 : « rapport d'import »).
type Report struct {
	FileName     string `json:"fileName"`
	Software     string `json:"software"`
	GedcomVers   string `json:"gedcomVersion"`
	Encoding     string `json:"encoding"`
	DeclaredEnc  string `json:"declaredEncoding"`
	Individuals  int    `json:"individuals"`
	Families     int    `json:"families"`
	Unions       int    `json:"unions"`
	ParentLinks  int    `json:"parentLinks"`
	Names        int    `json:"names"`
	Events       int    `json:"events"`
	Places       int    `json:"places"`
	Sources      int    `json:"sources"`
	Citations    int    `json:"citations"`
	Notes        int    `json:"notes"`
	ExtraTags    int    `json:"extraTags"`    // balises conservées telles quelles
	MediaSkipped int    `json:"mediaSkipped"` // objets multimédias non importés
	AutoDeceased int    `json:"autoDeceased"` // décédés par ancienneté (> 110 ans)
	// Skipped : balises ignorées → nombre d'occurrences
	Skipped         map[string]int `json:"skipped"`
	Warnings        []string       `json:"warnings"`
	WarningsDropped int            `json:"warningsDropped"` // avertissements non listés (plafond)
	Corrections     []string       `json:"corrections"`     // corrections automatiques appliquées
	DurationMs      int64          `json:"durationMs"`
}

const maxListedWarnings = 300

func (r *Report) warn(format string, a ...any) {
	if len(r.Warnings) >= maxListedWarnings {
		r.WarningsDropped++
		return
	}
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, a...))
}

func (r *Report) correct(format string, a ...any) {
	if len(r.Corrections) < maxListedWarnings {
		r.Corrections = append(r.Corrections, fmt.Sprintf(format, a...))
	}
}

func (r *Report) skip(tag string) {
	if r.Skipped == nil {
		r.Skipped = map[string]int{}
	}
	r.Skipped[tag]++
}
