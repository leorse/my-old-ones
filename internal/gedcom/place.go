package gedcom

import (
	"regexp"
	"strconv"
	"strings"
)

type placeLevel int

const (
	lvlNone placeLevel = iota
	lvlCountry
	lvlRegion
	lvlDepartment
	lvlCommune
	lvlSub // lieu-dit, adresse…
	lvlCode
)

var levelTypes = map[placeLevel]string{lvlCountry: "country", lvlRegion: "region",
	lvlDepartment: "department", lvlCommune: "commune", lvlSub: "lieu-dit"}

func fold(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "î", "i", "ï", "i",
		"ô", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c", "œ", "oe", "-", " ", "'", " ", "’", " ").Replace(s)
}

// formLevels interprète HEAD.PLAC.FORM (« Town, Area code, County, … »).
// Renvoie nil si le format est absent ou incompréhensible.
func formLevels(form string) []placeLevel {
	if strings.TrimSpace(form) == "" {
		return nil
	}
	var out []placeLevel
	known := 0
	for _, label := range strings.Split(form, ",") {
		l := fold(label)
		var lv placeLevel
		switch {
		case strings.Contains(l, "area code"), strings.Contains(l, "postal"), strings.Contains(l, "code postal"),
			l == "zip", strings.Contains(l, "insee"), l == "code":
			lv = lvlCode
		case l == "town", l == "city", l == "commune", l == "ville", l == "village", l == "localite", l == "lieu":
			lv = lvlCommune
		case l == "county", strings.HasPrefix(l, "departement"), l == "district", l == "province":
			lv = lvlDepartment
		case l == "region", l == "state", l == "etat":
			lv = lvlRegion
		case l == "country", l == "pays":
			lv = lvlCountry
		case l == "subdivision", strings.Contains(l, "lieu dit"), l == "address", l == "adresse", l == "street", l == "hamlet":
			lv = lvlSub
		}
		if lv != lvlNone {
			known++
		}
		out = append(out, lv)
	}
	if known < 2 {
		return nil
	}
	return out
}

var reInsee = regexp.MustCompile(`^(\d{5}|2[AB]\d{3})$`)

// PlaceStep est un maillon de la hiérarchie d'un lieu, du plus général au plus précis.
type PlaceStep struct {
	Type  string
	Name  string
	Code  string // code INSEE (communes)
	Level placeLevel
}

// ParsePlace découpe une valeur PLAC en maillons hiérarchiques. levels vient de
// formLevels (nil = format libre : « du plus précis au plus général »).
func ParsePlace(value string, levels []placeLevel) []PlaceStep {
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	var by [lvlCode + 1]string
	var order []placeLevel
	if levels != nil {
		for i, p := range parts {
			if p == "" || i >= len(levels) || levels[i] == lvlNone {
				continue
			}
			if by[levels[i]] == "" {
				by[levels[i]] = p
				order = append(order, levels[i])
			}
		}
		// un code non numérique est souvent le nom du département (« Chamniers, dordogne, , , France »)
		if c := by[lvlCode]; c != "" && !reInsee.MatchString(c) {
			if _, err := strconv.Atoi(c); err != nil && by[lvlDepartment] == "" {
				by[lvlDepartment] = c
			}
			by[lvlCode] = ""
		}
	} else {
		// format libre, du plus précis au plus général : le dernier élément est le pays
		var names []string
		for _, p := range parts {
			if p != "" {
				names = append(names, p)
			}
		}
		layouts := map[int][]placeLevel{ // du plus précis au plus général
			1: {lvlCommune},
			2: {lvlCommune, lvlCountry},
			3: {lvlCommune, lvlDepartment, lvlCountry},
			4: {lvlCommune, lvlDepartment, lvlRegion, lvlCountry},
			5: {lvlSub, lvlCommune, lvlDepartment, lvlRegion, lvlCountry},
		}
		if len(names) > 5 {
			names = append([]string{strings.Join(names[:len(names)-4], ", ")}, names[len(names)-4:]...)
		}
		for i, lv := range layouts[len(names)] {
			by[lv] = names[i]
		}
	}
	var steps []PlaceStep
	for _, lv := range []placeLevel{lvlCountry, lvlRegion, lvlDepartment, lvlCommune, lvlSub} {
		if by[lv] == "" {
			continue
		}
		st := PlaceStep{Type: levelTypes[lv], Name: by[lv], Level: lv}
		if lv == lvlCommune {
			st.Code = by[lvlCode]
		}
		steps = append(steps, st)
	}
	return steps
}

// ParseCoord lit « N45.7587 », « W0.474118 » (ou un nombre signé).
func ParseCoord(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	sign := 1.0
	switch s[0] {
	case 'S', 's', 'W', 'w':
		sign, s = -1, s[1:]
	case 'N', 'n', 'E', 'e':
		s = s[1:]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return sign * v, true
}
