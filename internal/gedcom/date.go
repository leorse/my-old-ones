package gedcom

import (
	"regexp"
	"strconv"
	"strings"

	"moo/internal/domain"
)

var gregMonths = map[string]int{"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12}

var frenchRMonths = map[string]int{"VEND": 1, "BRUM": 2, "FRIM": 3, "NIVO": 4, "PLUV": 5, "VENT": 6,
	"GERM": 7, "FLOR": 8, "PRAI": 9, "MESS": 10, "THER": 11, "FRUC": 12, "COMP": 13}

var (
	reCalEsc  = regexp.MustCompile(`^@#D([A-Z0-9 ]+)@\s*`)
	reBetween = regexp.MustCompile(`^BET(?:WEEN)?\s+(.+?)\s+AND\s+(.+)$`)
	reFromTo  = regexp.MustCompile(`^FROM\s+(.+?)\s+TO\s+(.+)$`)
	reDualYr  = regexp.MustCompile(`^(\d{1,4})/\d{1,4}$`)
)

type gdate struct {
	cal     domain.Calendar
	y, m, d int
}

func parseGedcomPoint(s string, defCal domain.Calendar) (gdate, bool) {
	s = strings.TrimSpace(s)
	cal := defCal
	if m := reCalEsc.FindStringSubmatch(s); m != nil {
		switch strings.TrimSpace(m[1]) {
		case "GREGORIAN":
			cal = domain.CalGregorian
		case "JULIAN":
			cal = domain.CalJulian
		case "FRENCH R":
			cal = domain.CalRepublican
		default: // hébraïque, etc. : non pris en charge
			return gdate{}, false
		}
		s = s[len(m[0]):]
	}
	f := strings.Fields(s)
	if len(f) == 0 || len(f) > 3 {
		return gdate{}, false
	}
	months := gregMonths
	if cal == domain.CalRepublican {
		months = frenchRMonths
	}
	year := func(t string) (int, bool) {
		if m := reDualYr.FindStringSubmatch(t); m != nil { // 1750/51 : double datation
			t = m[1]
		}
		n, err := strconv.Atoi(t)
		return n, err == nil && n > 0
	}
	var g gdate
	g.cal = cal
	var ok bool
	switch len(f) {
	case 1:
		if g.y, ok = year(f[0]); !ok {
			return gdate{}, false
		}
	case 2:
		if g.m, ok = months[strings.ToUpper(f[0])]; !ok {
			return gdate{}, false
		}
		if g.y, ok = year(f[1]); !ok {
			return gdate{}, false
		}
	case 3:
		d, err := strconv.Atoi(f[0])
		if err != nil || d < 1 {
			return gdate{}, false
		}
		g.d = d
		if g.m, ok = months[strings.ToUpper(f[1])]; !ok {
			return gdate{}, false
		}
		if g.y, ok = year(f[2]); !ok {
			return gdate{}, false
		}
	}
	return g, true
}

// ParseDate convertit une valeur DATE GEDCOM. Les formes non reconnues
// (« 07 ... 1792 », calendrier hébraïque, phrases, « B.C. ») deviennent des
// dates « texte libre » qui conservent la valeur d'origine. ok vaut false
// pour les dates structurées valides ; exact indique une conversion fidèle.
func ParseDate(raw string) domain.Date {
	text := strings.TrimSpace(raw)
	free := domain.Date{Precision: domain.PrecText, Calendar: domain.CalGregorian, OriginalText: text}
	if text == "" {
		return domain.Date{Precision: domain.PrecNone, Calendar: domain.CalGregorian}
	}
	up := strings.ToUpper(text)
	if strings.HasPrefix(up, "(") { // phrase libre GEDCOM
		return free
	}
	if strings.HasPrefix(up, "INT ") { // « INT date (phrase) » : la date interprétée
		up = strings.TrimSpace(up[4:])
		if i := strings.Index(up, "("); i >= 0 {
			up = strings.TrimSpace(up[:i])
		}
	}
	if strings.Contains(up, "B.C.") || strings.Contains(up, " BC") {
		return free
	}

	build := func(prec domain.Precision, a gdate, b *gdate) domain.Date {
		d := domain.Date{Precision: prec, Calendar: a.cal, Y1: a.y, M1: a.m, D1: a.d, OriginalText: text}
		if b != nil {
			if b.cal != a.cal {
				return free
			}
			d.Y2, d.M2, d.D2 = b.y, b.m, b.d
		}
		if d.Validate() != nil {
			return free
		}
		return d
	}
	cal := domain.CalGregorian
	// le marqueur de calendrier peut précéder un qualificatif ou une borne
	switch {
	case reBetween.MatchString(up):
		m := reBetween.FindStringSubmatch(up)
		a, ok1 := parseGedcomPoint(m[1], cal)
		b, ok2 := parseGedcomPoint(m[2], a.cal)
		if !ok1 || !ok2 {
			return free
		}
		return build(domain.PrecBetween, a, &b)
	case reFromTo.MatchString(up):
		m := reFromTo.FindStringSubmatch(up)
		a, ok1 := parseGedcomPoint(m[1], cal)
		b, ok2 := parseGedcomPoint(m[2], a.cal)
		if !ok1 || !ok2 {
			return free
		}
		return build(domain.PrecBetween, a, &b)
	}
	prec := domain.Precision("")
	rest := up
	for _, q := range []struct {
		prefix string
		prec   domain.Precision
	}{{"ABT", domain.PrecAbout}, {"ABOUT", domain.PrecAbout}, {"CAL", domain.PrecAbout}, {"EST", domain.PrecAbout},
		{"BEF", domain.PrecBefore}, {"BEFORE", domain.PrecBefore}, {"AFT", domain.PrecAfter}, {"AFTER", domain.PrecAfter},
		{"FROM", domain.PrecAfter}, {"TO", domain.PrecBefore}} {
		if strings.HasPrefix(rest, q.prefix+" ") {
			prec, rest = q.prec, strings.TrimSpace(rest[len(q.prefix):])
			break
		}
	}
	a, ok := parseGedcomPoint(rest, cal)
	if !ok {
		return free
	}
	if prec == "" {
		switch {
		case a.d > 0:
			prec = domain.PrecExact
		case a.m > 0:
			prec = domain.PrecMonth
		default:
			prec = domain.PrecYear
		}
	}
	return build(prec, a, nil)
}
