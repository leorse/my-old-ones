package domain

import "testing"

func TestDateValidateAndSortKey(t *testing.T) {
	d := Date{Precision: PrecExact, Calendar: CalGregorian, Y1: 1845, M1: 3, D1: 12}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if k, ok := d.SortKey(); !ok || k != 18450312 {
		t.Fatalf("clé %d %v", k, ok)
	}
	if err := (Date{Precision: PrecExact, Y1: 1845, D1: 5}).Validate(); err == nil {
		t.Error("jour sans mois accepté")
	}
	if err := (Date{Precision: PrecBetween, Y1: 1840}).Validate(); err == nil {
		t.Error("intervalle sans fin accepté")
	}
	if err := (Date{Precision: PrecExact, Y1: 1845, M1: 2, D1: 30}).Validate(); err == nil {
		t.Error("30 février accepté")
	}
	// année républicaine seule : clé = année grégorienne de début
	if k, ok := (Date{Precision: PrecYear, Calendar: CalRepublican, Y1: 12}).SortKey(); !ok || k != 18030000 {
		t.Errorf("an XII: %d %v", k, ok)
	}
}

func TestRepublicanConversion(t *testing.T) {
	cases := []struct{ y, m, d, gy, gm, gd int }{
		{1, 1, 1, 1792, 9, 22},
		{2, 1, 1, 1793, 9, 22},
		{2, 4, 1, 1793, 12, 21},
		{8, 2, 18, 1799, 11, 9}, // 18 brumaire an VIII
		{12, 7, 3, 1804, 3, 24}, // 3 germinal an XII
		{3, 13, 6, 1795, 9, 22}, // 6e jour complémentaire (an III sextile)
		{14, 4, 10, 1805, 12, 31},
	}
	for _, c := range cases {
		gy, gm, gd, err := RepublicanToGregorian(c.y, c.m, c.d)
		if err != nil || gy != c.gy || gm != c.gm || gd != c.gd {
			t.Errorf("%d/%d/%d: %d-%d-%d (%v), attendu %d-%d-%d", c.d, c.m, c.y, gy, gm, gd, err, c.gy, c.gm, c.gd)
			continue
		}
		y, m, d, err := GregorianToRepublican(gy, gm, gd)
		if err != nil || y != c.y || m != c.m || d != c.d {
			t.Errorf("aller-retour %d-%d-%d: %d/%d/%d (%v)", gy, gm, gd, d, m, y, err)
		}
	}
	if _, _, _, err := RepublicanToGregorian(4, 13, 6); err == nil {
		t.Error("6e jour complémentaire d'une année non sextile accepté")
	}
	if _, _, _, err := RepublicanToGregorian(15, 1, 1); err == nil {
		t.Error("an XV accepté")
	}
	if _, _, _, err := GregorianToRepublican(1806, 1, 1); err == nil {
		t.Error("1806 accepté")
	}
}

func TestJulianToGregorian(t *testing.T) {
	if y, m, d := JulianToGregorian(1582, 10, 5); y != 1582 || m != 10 || d != 15 {
		t.Errorf("5 oct 1582 julien = %d-%d-%d", y, m, d)
	}
}

func TestRoman(t *testing.T) {
	for n := 1; n <= 14; n++ {
		if RomanToInt(IntToRoman(n)) != n {
			t.Errorf("romain %d", n)
		}
	}
	if RomanToInt("xii") != 12 || RomanToInt("abc") != 0 {
		t.Error("RomanToInt")
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want Date
	}{
		{"12/3/1845", Date{Precision: PrecExact, Calendar: CalGregorian, Y1: 1845, M1: 3, D1: 12}},
		{"12 mars 1845", Date{Precision: PrecExact, Calendar: CalGregorian, Y1: 1845, M1: 3, D1: 12}},
		{"1er février 1900", Date{Precision: PrecExact, Calendar: CalGregorian, Y1: 1900, M1: 2, D1: 1}},
		{"août 1870", Date{Precision: PrecMonth, Calendar: CalGregorian, Y1: 1870, M1: 8}},
		{"03/1870", Date{Precision: PrecMonth, Calendar: CalGregorian, Y1: 1870, M1: 3}},
		{"1845", Date{Precision: PrecYear, Calendar: CalGregorian, Y1: 1845}},
		{"vers 1845", Date{Precision: PrecAbout, Calendar: CalGregorian, Y1: 1845}},
		{"Environ mars 1845", Date{Precision: PrecAbout, Calendar: CalGregorian, Y1: 1845, M1: 3}},
		{"avant 1850", Date{Precision: PrecBefore, Calendar: CalGregorian, Y1: 1850}},
		{"après le", Date{Precision: PrecText, Calendar: CalGregorian}},
		{"ap. 1850", Date{Precision: PrecAfter, Calendar: CalGregorian, Y1: 1850}},
		{"entre 1840 et 1845", Date{Precision: PrecBetween, Calendar: CalGregorian, Y1: 1840, Y2: 1845}},
		{"entre mars 1840 et 12/5/1841", Date{Precision: PrecBetween, Calendar: CalGregorian, Y1: 1840, M1: 3, Y2: 1841, M2: 5, D2: 12}},
		{"3 germinal an XII", Date{Precision: PrecExact, Calendar: CalRepublican, Y1: 12, M1: 7, D1: 3}},
		{"18 Brumaire an 8", Date{Precision: PrecExact, Calendar: CalRepublican, Y1: 8, M1: 2, D1: 18}},
		{"floréal an III", Date{Precision: PrecMonth, Calendar: CalRepublican, Y1: 3, M1: 8}},
		{"an XII", Date{Precision: PrecYear, Calendar: CalRepublican, Y1: 12}},
	}
	for _, c := range cases {
		got := ParseDate(c.in)
		if c.want.Precision == PrecText {
			if got.Precision != PrecText || got.OriginalText != c.in {
				t.Errorf("%q: attendu texte libre, obtenu %+v", c.in, got)
			}
			continue
		}
		got.OriginalText = ""
		if got != c.want {
			t.Errorf("%q:\n got  %+v\n want %+v", c.in, got, c.want)
		}
	}
}

func TestParseDateFallsBackToText(t *testing.T) {
	for _, in := range []string{"30 février 1845", "n'importe quoi", "entre 1850 et 1840", "5 an XII", "31/13/1845", "3 floreal an XV"} {
		if d := ParseDate(in); d.Precision != PrecText || d.OriginalText != in {
			t.Errorf("%q devrait rester du texte libre: %+v", in, d)
		}
	}
	if d := ParseDate("  "); d.Precision != PrecNone {
		t.Errorf("vide: %+v", d)
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		in    string
		style DateStyle
		want  string
	}{
		{"12/3/1845", StyleLong, "12 mars 1845"},
		{"12/3/1845", StyleNumeric, "12/03/1845"},
		{"1/2/1900", StyleLong, "1er février 1900"},
		{"vers 1845", StyleLong, "vers 1845"},
		{"entre 1840 et 1845", StyleLong, "entre 1840 et 1845"},
		{"après mars 1850", StyleLong, "après mars 1850"},
		{"3 germinal an XII", StyleLong, "3 germinal an XII (24 mars 1804)"},
		{"germinal an XII", StyleLong, "germinal an XII"},
		{"texte libre bizarre", StyleLong, "texte libre bizarre"},
	}
	for _, c := range cases {
		if got := ParseDate(c.in).Format(c.style); got != c.want {
			t.Errorf("%q: %q, attendu %q", c.in, got, c.want)
		}
	}
	// aller-retour : le format long se relit identiquement
	d := ParseDate("12 mars 1845")
	if r := ParseDate(d.Format(StyleLong)); r.Y1 != d.Y1 || r.M1 != d.M1 || r.D1 != d.D1 {
		t.Errorf("aller-retour: %+v", r)
	}
}
