package domain

import (
	"fmt"
	"strings"
)

// Conversions de calendriers via le numéro de jour julien (JDN).

func gregorianToJDN(y, m, d int) int {
	a := (14 - m) / 12
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + (153*mm+2)/5 + 365*yy + yy/4 - yy/100 + yy/400 - 32045
}

func jdnToGregorian(j int) (y, m, d int) {
	a := j + 32044
	b := (4*a + 3) / 146097
	c := a - 146097*b/4
	dd := (4*c + 3) / 1461
	e := c - 1461*dd/4
	mm := (5*e + 2) / 153
	d = e - (153*mm+2)/5 + 1
	m = mm + 3 - 12*(mm/10)
	y = 100*b + dd - 4800 + mm/10
	return
}

func julianToJDN(y, m, d int) int {
	a := (14 - m) / 12
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + (153*mm+2)/5 + 365*yy + yy/4 - 32083
}

// JulianToGregorian convertit une date julienne en grégorienne.
func JulianToGregorian(y, m, d int) (int, int, int) {
	return jdnToGregorian(julianToJDN(y, m, d))
}

func isGregorianLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func daysInGregorianMonth(y, m int) int {
	switch m {
	case 2:
		if isGregorianLeap(y) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// Calendrier républicain : début historique de chaque année (1er vendémiaire),
// de l'an I (1792) à l'an XIV (1805). Le calendrier est aboli le 1er janvier 1806.
var republicanYearStart = [15][3]int{
	{}, // inutilisé (les années commencent à 1)
	{1792, 9, 22}, {1793, 9, 22}, {1794, 9, 22}, {1795, 9, 23}, {1796, 9, 22},
	{1797, 9, 22}, {1798, 9, 22}, {1799, 9, 23}, {1800, 9, 23}, {1801, 9, 23},
	{1802, 9, 23}, {1803, 9, 24}, {1804, 9, 23}, {1805, 9, 23},
}

var republicanEndJDN = gregorianToJDN(1805, 12, 31)

var RepublicanMonths = [14]string{"", "vendémiaire", "brumaire", "frimaire", "nivôse", "pluviôse",
	"ventôse", "germinal", "floréal", "prairial", "messidor", "thermidor", "fructidor",
	"jour complémentaire"}

func republicanStartJDN(y int) int {
	s := republicanYearStart[y]
	return gregorianToJDN(s[0], s[1], s[2])
}

func republicanYearLength(y int) int {
	if y >= 14 {
		return 365
	}
	return republicanStartJDN(y+1) - republicanStartJDN(y)
}

// RepublicanToGregorian convertit une date républicaine (mois 1..13).
func RepublicanToGregorian(y, m, d int) (gy, gm, gd int, err error) {
	if y < 1 || y > 14 {
		return 0, 0, 0, fmt.Errorf("an %d hors du calendrier républicain (I à XIV)", y)
	}
	maxDay := 30
	if m == 13 {
		maxDay = republicanYearLength(y) - 360
	}
	if m < 1 || m > 13 || d < 1 || d > maxDay {
		return 0, 0, 0, fmt.Errorf("date républicaine invalide: %d/%d/%d", d, m, y)
	}
	j := republicanStartJDN(y) + (m-1)*30 + d - 1
	if j > republicanEndJDN {
		return 0, 0, 0, fmt.Errorf("date postérieure à l'abolition du calendrier républicain")
	}
	gy, gm, gd = jdnToGregorian(j)
	return gy, gm, gd, nil
}

// GregorianToRepublican convertit une date grégorienne de la période 1792–1805.
func GregorianToRepublican(gy, gm, gd int) (y, m, d int, err error) {
	j := gregorianToJDN(gy, gm, gd)
	if j < republicanStartJDN(1) || j > republicanEndJDN {
		return 0, 0, 0, fmt.Errorf("hors de la période républicaine")
	}
	for y = 14; y >= 1; y-- {
		if j >= republicanStartJDN(y) {
			break
		}
	}
	off := j - republicanStartJDN(y)
	return y, off/30 + 1, off%30 + 1, nil
}

var romanValues = map[byte]int{'i': 1, 'v': 5, 'x': 10, 'l': 50, 'c': 100}

// RomanToInt convertit un chiffre romain (minuscules ou majuscules) ; 0 si invalide.
func RomanToInt(s string) int {
	s = strings.ToLower(s)
	if s == "" {
		return 0
	}
	total := 0
	for i := 0; i < len(s); i++ {
		v, ok := romanValues[s[i]]
		if !ok {
			return 0
		}
		if i+1 < len(s) && romanValues[s[i+1]] > v {
			total -= v
		} else {
			total += v
		}
	}
	return total
}

// IntToRoman écrit n (1..399) en chiffres romains.
func IntToRoman(n int) string {
	vals := []int{100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}
