package domain

// AgeYears calcule l'âge en années révolues entre deux dates (calendriers
// convertis en grégorien). ok vaut false si l'une des dates est inexploitable
// ou si to précède from. approx est vrai quand l'âge est incertain : précision
// insuffisante (année seule, mois seul) ou date qualifiée (vers/avant/après/entre).
func AgeYears(from, to Date) (years int, approx bool, ok bool) {
	if !from.Structured() || !to.Structured() {
		return 0, false, false
	}
	y1, m1, d1, ok1 := toGregorian(from.Calendar, from.Y1, from.M1, from.D1)
	y2, m2, d2, ok2 := toGregorian(to.Calendar, to.Y1, to.M1, to.D1)
	if !ok1 || !ok2 {
		return 0, false, false
	}
	if y2*10000+m2*100+d2 < y1*10000+m1*100+d1 {
		return 0, false, false
	}
	years = y2 - y1
	known := m1 > 0 && m2 > 0
	if known && (m2 < m1 || (m2 == m1 && d1 > 0 && d2 > 0 && d2 < d1)) {
		years--
	}
	approx = !known || (m1 == m2 && (d1 == 0 || d2 == 0))
	for _, d := range []Date{from, to} {
		switch d.Precision {
		case PrecAbout, PrecBefore, PrecAfter, PrecBetween, PrecYear, PrecMonth:
			approx = true
		}
	}
	if years < 0 {
		return 0, false, false
	}
	return years, approx, true
}
