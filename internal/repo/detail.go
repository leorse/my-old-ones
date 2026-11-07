package repo

import (
	"context"
	"strings"

	"moo/internal/domain"
)

// EventAge : âge de la personne à la date d'un événement.
type EventAge struct {
	EventID string `json:"eventId"`
	Years   int    `json:"years"`
	Approx  bool   `json:"approx"`
}

// PersonDetail regroupe tout ce que la fiche affiche (spec §5.3).
type PersonDetail struct {
	Core   PersonCore `json:"core"`
	Events []EventDTO `json:"events"`
	Family Family     `json:"family"`
	// Calculs affichés
	Ages       []EventAge `json:"ages"`       // âge à chaque événement (après la naissance)
	AgeAtDeath *EventAge  `json:"ageAtDeath"` // nil si inconnu
	Sosa       []int64    `json:"sosa"`       // numéros Sosa depuis la racine (vide si hors lignée)
	IsRoot     bool       `json:"isRoot"`
	ParentAges []EventAge `json:"parentAges"` // âge de chaque parent à la naissance de la personne (EventID = id du parent)
}

func firstEvent(evs []EventDTO, typ string) *EventDTO {
	for i := range evs {
		if evs[i].Type == typ && evs[i].Date.Structured() {
			return &evs[i]
		}
	}
	return nil
}

func (s *Store) GetPersonDetail(ctx context.Context, id string) (PersonDetail, error) {
	var d PersonDetail
	var err error
	if d.Core, err = s.GetCore(ctx, id); err != nil {
		return d, err
	}
	if d.Events, err = s.PersonEvents(ctx, id); err != nil {
		return d, err
	}
	if d.Family, err = s.GetFamily(ctx, id); err != nil {
		return d, err
	}
	d.Ages, d.ParentAges, d.Sosa = []EventAge{}, []EventAge{}, []int64{}

	birth := firstEvent(d.Events, "birth")
	if birth != nil {
		for _, e := range d.Events {
			if e.ID == birth.ID || !e.Date.Structured() {
				continue
			}
			if y, ap, ok := domain.AgeYears(birth.Date, e.Date); ok {
				a := EventAge{EventID: e.ID, Years: y, Approx: ap}
				d.Ages = append(d.Ages, a)
				if e.Type == "death" && d.AgeAtDeath == nil {
					c := a
					d.AgeAtDeath = &c
				}
			}
		}
		for _, p := range d.Family.Parents {
			pev, err := s.PersonEvents(ctx, p.Person.ID)
			if err != nil {
				return d, err
			}
			if pb := firstEvent(pev, "birth"); pb != nil {
				if y, ap, ok := domain.AgeYears(pb.Date, birth.Date); ok {
					d.ParentAges = append(d.ParentAges, EventAge{EventID: p.Person.ID, Years: y, Approx: ap})
				}
			}
		}
	}

	root, err := s.Root(ctx)
	if err != nil {
		return d, err
	}
	d.IsRoot = root == id
	if root != "" {
		nums, err := s.SosaNumbers(ctx, root, 0)
		if err != nil {
			return d, err
		}
		if ns, ok := nums[id]; ok {
			d.Sosa = ns
		}
	}
	return d, nil
}

// SosaEntry est une ligne de la liste Sosa.
type SosaEntry struct {
	Number int64         `json:"number"`
	Person PersonSummary `json:"person"`
}

// SosaList renvoie les ancêtres numérotés de la racine, triés par numéro
// (un ancêtre en implexe apparaît une fois par numéro).
func (s *Store) SosaList(ctx context.Context) ([]SosaEntry, error) {
	root, err := s.Root(ctx)
	if err != nil || root == "" {
		return []SosaEntry{}, err
	}
	nums, err := s.SosaNumbers(ctx, root, 0)
	if err != nil {
		return nil, err
	}
	out := []SosaEntry{}
	for id, ns := range nums {
		sum, err := s.GetSummary(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, n := range ns {
			out = append(out, SosaEntry{Number: n, Person: sum})
		}
	}
	sortSosa(out)
	return out, nil
}

func sortSosa(e []SosaEntry) {
	for i := 1; i < len(e); i++ {
		for j := i; j > 0 && e[j].Number < e[j-1].Number; j-- {
			e[j], e[j-1] = e[j-1], e[j]
		}
	}
}

// SearchPersons cherche dans les noms (tous types), les notes et les lieux
// d'événements. Insensible à la casse ASCII ; la recherche phonétique et
// l'index FTS5 viendront ensuite (spec §5.5).
func (s *Store) SearchPersons(ctx context.Context, query string, limit int) ([]PersonSummary, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []PersonSummary{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	// chaque mot doit se retrouver quelque part (nom, notes ou lieu)
	where := []string{}
	args := []any{}
	for _, w := range strings.Fields(q) {
		like := "%" + esc.Replace(w) + "%"
		where = append(where, `(EXISTS (SELECT 1 FROM names n WHERE n.person_id = p.id AND (n.surname LIKE ? ESCAPE '\' OR n.given_names LIKE ? ESCAPE '\'))
			OR p.notes_md LIKE ? ESCAPE '\'
			OR EXISTS (SELECT 1 FROM events e JOIN places pl ON pl.id = e.place_id WHERE e.person_id = p.id AND pl.display_name LIKE ? ESCAPE '\'))`)
		args = append(args, like, like, like, like)
	}
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.id, COALESCE(n.surname,''), COALESCE(n.given_names,'[]'), p.sex, p.living
		FROM persons p LEFT JOIN names n ON n.person_id = p.id AND n.is_primary = 1
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY n.surname COLLATE NOCASE, n.given_names COLLATE NOCASE LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonSummary{}
	for rows.Next() {
		var p PersonSummary
		var given string
		if err := rows.Scan(&p.ID, &p.Surname, &given, &p.Sex, &p.Living); err != nil {
			return nil, err
		}
		p.GivenNames = parseGiven(given)
		out = append(out, p)
	}
	return out, rows.Err()
}
