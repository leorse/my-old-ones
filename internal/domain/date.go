// Package domain contient les types et règles métier indépendantes de la
// persistance et de l'interface (spec §4).
package domain

import "fmt"

type Precision string

const (
	PrecNone    Precision = "none"
	PrecExact   Precision = "exact"
	PrecMonth   Precision = "month"
	PrecYear    Precision = "year"
	PrecAbout   Precision = "about"
	PrecBefore  Precision = "before"
	PrecAfter   Precision = "after"
	PrecBetween Precision = "between"
	PrecText    Precision = "text"
)

type Calendar string

const (
	CalGregorian  Calendar = "gregorian"
	CalRepublican Calendar = "republican"
	CalJulian     Calendar = "julian"
)

// Date est la date généalogique étendue (spec §4.3). Les valeurs 0 signifient
// « non renseigné » pour mois et jour. Pour une date républicaine, y/m/d sont
// exprimés dans ce calendrier (mois 1..13).
type Date struct {
	Precision    Precision `json:"precision"`
	Calendar     Calendar  `json:"calendar"`
	Y1           int       `json:"y1"`
	M1           int       `json:"m1"`
	D1           int       `json:"d1"`
	Y2           int       `json:"y2"`
	M2           int       `json:"m2"`
	D2           int       `json:"d2"`
	OriginalText string    `json:"originalText"`
}

// Structured indique que la date a des composantes exploitables.
func (d Date) Structured() bool {
	return d.Precision != PrecNone && d.Precision != PrecText && d.Precision != ""
}

// Validate vérifie la cohérence structurelle et calendaire.
func (d Date) Validate() error {
	if !d.Structured() {
		return nil
	}
	if d.Y1 == 0 {
		return fmt.Errorf("année manquante")
	}
	if d.M1 < 0 || d.M1 > 13 || d.D1 < 0 || d.D1 > 31 || d.M2 < 0 || d.M2 > 13 || d.D2 < 0 || d.D2 > 31 {
		return fmt.Errorf("mois ou jour hors limites")
	}
	if d.D1 > 0 && d.M1 == 0 || d.D2 > 0 && d.M2 == 0 {
		return fmt.Errorf("jour sans mois")
	}
	if d.Precision == PrecBetween && d.Y2 == 0 {
		return fmt.Errorf("borne de fin manquante")
	}
	switch d.Calendar {
	case CalRepublican:
		if d.M1 > 0 && d.D1 > 0 {
			if _, _, _, err := RepublicanToGregorian(d.Y1, d.M1, d.D1); err != nil {
				return err
			}
		}
	default:
		if d.M1 > 12 || d.M2 > 12 {
			return fmt.Errorf("mois hors limites")
		}
		if d.Calendar != CalJulian && d.D1 > 0 && d.D1 > daysInGregorianMonth(d.Y1, d.M1) {
			return fmt.Errorf("jour invalide pour ce mois")
		}
	}
	return nil
}

// toGregorian ramène (y,m,d) du calendrier cal vers le grégorien. Les mois ou
// jours manquants sont remplacés par 1 pour la conversion et restitués à 0.
func toGregorian(cal Calendar, y, m, d int) (int, int, int, bool) {
	if cal == "" || cal == CalGregorian {
		return y, m, d, true
	}
	mm, dd := m, d
	if mm == 0 {
		mm = 1
	}
	if dd == 0 {
		dd = 1
	}
	var gy, gm, gd int
	switch cal {
	case CalJulian:
		gy, gm, gd = JulianToGregorian(y, mm, dd)
	case CalRepublican:
		var err error
		gy, gm, gd, err = RepublicanToGregorian(y, mm, dd)
		if err != nil {
			return 0, 0, 0, false
		}
	default:
		return 0, 0, 0, false
	}
	if m == 0 {
		gm, gd = 0, 0
	} else if d == 0 {
		gd = 0
	}
	return gy, gm, gd, true
}

// SortKey donne un entier de tri AAAAMMJJ en grégorien (calendriers julien et
// républicain convertis). ok vaut false sans date exploitable.
func (d Date) SortKey() (key int64, ok bool) {
	if !d.Structured() {
		return 0, false
	}
	y, m, dd, ok := toGregorian(d.Calendar, d.Y1, d.M1, d.D1)
	if !ok {
		return 0, false
	}
	return int64(y)*10000 + int64(m)*100 + int64(dd), true
}

// GregorianYear renvoie l'année grégorienne de la date principale.
func (d Date) GregorianYear() (int, bool) {
	if !d.Structured() {
		return 0, false
	}
	y, _, _, ok := toGregorian(d.Calendar, d.Y1, d.M1, d.D1)
	return y, ok
}
