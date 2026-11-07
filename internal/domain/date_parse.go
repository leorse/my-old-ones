package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var accentReplacer = strings.NewReplacer(
	"é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a",
	"î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c",
)

func normalizeDateText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = accentReplacer.Replace(s)
	s = strings.ReplaceAll(s, "1er", "1")
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

var gregorianMonthNames = map[string]int{
	"janvier": 1, "janv": 1, "jan": 1, "fevrier": 2, "fevr": 2, "fev": 2, "mars": 3, "mar": 3,
	"avril": 4, "avr": 4, "mai": 5, "juin": 6, "juillet": 7, "juil": 7, "aout": 8,
	"septembre": 9, "sept": 9, "sep": 9, "octobre": 10, "oct": 10,
	"novembre": 11, "nov": 11, "decembre": 12, "dec": 12,
}

var republicanMonthNames = map[string]int{
	"vendemiaire": 1, "brumaire": 2, "frimaire": 3, "nivose": 4, "pluviose": 5, "ventose": 6,
	"germinal": 7, "floreal": 8, "prairial": 9, "messidor": 10, "thermidor": 11, "fructidor": 12,
	"sansculottides": 13, "complementaire": 13, "complementaires": 13,
}

var (
	reNumeric  = regexp.MustCompile(`^(\d{1,2})[/\-.](\d{1,2})[/\-.](\d{3,4})$`)
	reMonthNum = regexp.MustCompile(`^(\d{1,2})[/\-.](\d{3,4})$`)
	reYear     = regexp.MustCompile(`^\d{3,4}$`)
	reWords    = regexp.MustCompile(`^(?:(\d{1,2}) )?(?:([a-z]+) )?(\d{3,4})$`)
	reRepub    = regexp.MustCompile(`^(?:(\d{1,2}) )?(?:([a-z]+) )?an ([ivxlc]+|\d{1,2})$`)
	reAbout    = regexp.MustCompile(`^(?:vers|environ|env\.?|circa|ca\.?|c\.|~) ?(.+)$`)
	reBefore   = regexp.MustCompile(`^(?:avant|av\.?) (.+)$`)
	reAfter    = regexp.MustCompile(`^(?:apres|ap\.?) (.+)$`)
	reBetween  = regexp.MustCompile(`^entre (.+?) et (.+)$`)
)

type ymd struct {
	cal     Calendar
	y, m, d int
}

func (p ymd) precision() Precision {
	switch {
	case p.d > 0:
		return PrecExact
	case p.m > 0:
		return PrecMonth
	}
	return PrecYear
}

// ParseDate interprète une saisie libre en français. Elle ne renvoie jamais
// d'erreur : une saisie incomprise ou invalide devient une date « texte libre »
// conservée telle quelle (spec §4.3).
func ParseDate(input string) Date {
	text := strings.TrimSpace(input)
	if text == "" {
		return Date{Precision: PrecNone, Calendar: CalGregorian}
	}
	n := normalizeDateText(text)
	freeText := Date{Precision: PrecText, Calendar: CalGregorian, OriginalText: text}

	prec := Precision("")
	rest := n
	switch {
	case reBetween.MatchString(n):
		m := reBetween.FindStringSubmatch(n)
		a, ok1 := parseSingle(m[1])
		b, ok2 := parseSingle(m[2])
		if !ok1 || !ok2 || a.cal != b.cal {
			return freeText
		}
		d := Date{Precision: PrecBetween, Calendar: a.cal, Y1: a.y, M1: a.m, D1: a.d,
			Y2: b.y, M2: b.m, D2: b.d, OriginalText: text}
		if d.Validate() != nil || endBeforeStart(a, b) {
			return freeText
		}
		return d
	case reAbout.MatchString(n):
		prec, rest = PrecAbout, reAbout.FindStringSubmatch(n)[1]
	case reBefore.MatchString(n):
		prec, rest = PrecBefore, reBefore.FindStringSubmatch(n)[1]
	case reAfter.MatchString(n):
		prec, rest = PrecAfter, reAfter.FindStringSubmatch(n)[1]
	}
	p, ok := parseSingle(rest)
	if !ok {
		return freeText
	}
	if prec == "" {
		prec = p.precision()
	}
	d := Date{Precision: prec, Calendar: p.cal, Y1: p.y, M1: p.m, D1: p.d, OriginalText: text}
	if d.Validate() != nil {
		return freeText
	}
	return d
}

func endBeforeStart(a, b ymd) bool {
	da := Date{Precision: PrecExact, Calendar: a.cal, Y1: a.y, M1: a.m, D1: a.d}
	db := Date{Precision: PrecExact, Calendar: b.cal, Y1: b.y, M1: b.m, D1: b.d}
	ka, _ := da.SortKey()
	kb, _ := db.SortKey()
	return kb < ka
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func parseSingle(s string) (ymd, bool) {
	s = strings.TrimSpace(s)
	if m := reNumeric.FindStringSubmatch(s); m != nil {
		return ymd{CalGregorian, atoi(m[3]), atoi(m[2]), atoi(m[1])}, true
	}
	if m := reMonthNum.FindStringSubmatch(s); m != nil {
		return ymd{CalGregorian, atoi(m[2]), atoi(m[1]), 0}, true
	}
	if reYear.MatchString(s) {
		return ymd{CalGregorian, atoi(s), 0, 0}, true
	}
	if m := reRepub.FindStringSubmatch(s); m != nil {
		year := RomanToInt(m[3])
		if year == 0 {
			year = atoi(m[3])
		}
		mo := 0
		if m[2] != "" {
			var ok bool
			if mo, ok = republicanMonthNames[m[2]]; !ok {
				return ymd{}, false
			}
		}
		d := atoi(m[1])
		if d > 0 && mo == 0 {
			return ymd{}, false
		}
		return ymd{CalRepublican, year, mo, d}, true
	}
	if m := reWords.FindStringSubmatch(s); m != nil {
		mo := 0
		if m[2] != "" {
			var ok bool
			if mo, ok = gregorianMonthNames[strings.TrimSuffix(m[2], ".")]; !ok {
				return ymd{}, false
			}
		}
		d := atoi(m[1])
		if d > 0 && mo == 0 {
			return ymd{}, false
		}
		return ymd{CalGregorian, atoi(m[3]), mo, d}, true
	}
	return ymd{}, false
}

// --- Affichage -------------------------------------------------------------

// DateStyle choisit le format d'affichage (préférence du projet).
type DateStyle string

const (
	StyleLong    DateStyle = "long"    // 12 mars 1845
	StyleNumeric DateStyle = "numeric" // 12/03/1845
)

var frMonths = [13]string{"", "janvier", "février", "mars", "avril", "mai", "juin", "juillet",
	"août", "septembre", "octobre", "novembre", "décembre"}

func fmtGregorian(y, m, d int, style DateStyle) string {
	switch {
	case style == StyleNumeric && d > 0:
		return fmt.Sprintf("%02d/%02d/%d", d, m, y)
	case style == StyleNumeric && m > 0:
		return fmt.Sprintf("%02d/%d", m, y)
	case d > 0:
		day := strconv.Itoa(d)
		if d == 1 {
			day = "1er"
		}
		return fmt.Sprintf("%s %s %d", day, frMonths[m], y)
	case m > 0:
		return fmt.Sprintf("%s %d", frMonths[m], y)
	}
	return strconv.Itoa(y)
}

func fmtComponent(cal Calendar, y, m, d int, style DateStyle) string {
	switch cal {
	case CalRepublican:
		out := "an " + IntToRoman(y)
		if m > 0 {
			out = RepublicanMonths[m] + " " + out
		}
		if d > 0 {
			out = strconv.Itoa(d) + " " + out
		}
		return out
	case CalJulian:
		return fmtGregorian(y, m, d, style) + " (julien)"
	}
	return fmtGregorian(y, m, d, style)
}

// Format affiche la date. Une date républicaine complète est suivie de son
// équivalent grégorien (spec §4.3 : « affichage des deux »).
func (d Date) Format(style DateStyle) string {
	switch d.Precision {
	case PrecNone, "":
		return ""
	case PrecText:
		return d.OriginalText
	}
	first := fmtComponent(d.Calendar, d.Y1, d.M1, d.D1, style)
	var out string
	switch d.Precision {
	case PrecAbout:
		out = "vers " + first
	case PrecBefore:
		out = "avant " + first
	case PrecAfter:
		out = "après " + first
	case PrecBetween:
		out = "entre " + first + " et " + fmtComponent(d.Calendar, d.Y2, d.M2, d.D2, style)
	default:
		out = first
	}
	if d.Calendar == CalRepublican && d.Precision != PrecBetween && d.M1 > 0 && d.D1 > 0 {
		if gy, gm, gd, ok := toGregorian(d.Calendar, d.Y1, d.M1, d.D1); ok {
			out += " (" + fmtGregorian(gy, gm, gd, style) + ")"
		}
	}
	return out
}
