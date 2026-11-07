package domain

import "testing"

func TestAgeYears(t *testing.T) {
	cases := []struct {
		from, to string
		years    int
		approx   bool
		ok       bool
	}{
		{"12/3/1845", "11/3/1900", 54, false, true},
		{"12/3/1845", "12/3/1900", 55, false, true},
		{"12/3/1845", "13/3/1900", 55, false, true},
		{"12/3/1845", "1/1/1900", 54, false, true},
		{"1845", "1900", 55, true, true},
		{"vers 1845", "1/1/1900", 55, true, true},
		{"mars 1845", "mars 1900", 55, true, true},
		{"3 germinal an XII", "24/3/1805", 1, false, true},
		{"1900", "1845", 0, false, false},
		{"texte", "1900", 0, false, false},
	}
	for _, c := range cases {
		y, ap, ok := AgeYears(ParseDate(c.from), ParseDate(c.to))
		if y != c.years || ap != c.approx || ok != c.ok {
			t.Errorf("%q -> %q : %d approx=%v ok=%v ; attendu %d %v %v", c.from, c.to, y, ap, ok, c.years, c.approx, c.ok)
		}
	}
}
