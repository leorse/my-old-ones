package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrCycle : le lien créerait un cycle généalogique (spec §4.5).
var ErrCycle = errors.New("un ancêtre ne peut pas être son propre descendant")

// ErrSelfParent : personne parent d'elle-même.
var ErrSelfParent = errors.New("une personne ne peut pas être son propre parent")

func NewID() string { return uuid.NewString() }

// NewPerson décrit une personne à créer avec son nom principal.
type NewPerson struct {
	Surname    string
	GivenNames []string
	Sex        string // M, F, other, unknown (défaut)
}

// PersonSummary est une ligne de liste de personnes.
type PersonSummary struct {
	ID         string   `json:"id"`
	Surname    string   `json:"surname"`
	GivenNames []string `json:"givenNames"`
	Sex        string   `json:"sex"`
	Living     string   `json:"living"`
}

// Display renvoie « Prénoms NOM ».
func (p PersonSummary) Display() string {
	return strings.TrimSpace(strings.Join(p.GivenNames, " ") + " " + strings.ToUpper(p.Surname))
}

func (t *Tx) createPerson(p NewPerson) (string, error) {
	if p.Sex == "" {
		p.Sex = "unknown"
	}
	if p.GivenNames == nil {
		p.GivenNames = []string{}
	}
	given, _ := json.Marshal(p.GivenNames)
	id := NewID()
	if err := t.Insert("persons", Row{
		"id": id, "sex": p.Sex, "created_at": t.Now(), "updated_at": t.Now(),
	}); err != nil {
		return "", err
	}
	return id, t.Insert("names", Row{
		"id": NewID(), "person_id": id, "type": "birth",
		"surname": p.Surname, "given_names": string(given), "is_primary": 1,
	})
}

// touchPerson met à jour updated_at (sans journaliser un changement vide).
func (t *Tx) touchPerson(id string) error {
	return t.Update("persons", id, Row{"updated_at": t.Now()})
}

// CreatePerson crée une personne et son nom principal en une opération.
func (s *Store) CreatePerson(ctx context.Context, p NewPerson) (string, error) {
	var id string
	_, err := s.Mutate(ctx, "Créer une personne", func(t *Tx) (err error) {
		id, err = t.createPerson(p)
		return
	})
	return id, err
}

func parseGiven(s string) []string {
	var g []string
	if err := json.Unmarshal([]byte(s), &g); err != nil || g == nil {
		return []string{}
	}
	return g
}

// ListPersons renvoie les personnes avec leur nom principal, triées par nom.
func (s *Store) ListPersons(ctx context.Context) ([]PersonSummary, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.id, COALESCE(n.surname,''), COALESCE(n.given_names,'[]'), p.sex, p.living
		FROM persons p LEFT JOIN names n ON n.person_id = p.id AND n.is_primary = 1
		ORDER BY n.surname COLLATE NOCASE, n.given_names COLLATE NOCASE`)
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

// GetSummary renvoie le résumé d'une personne.
func (s *Store) GetSummary(ctx context.Context, id string) (PersonSummary, error) {
	var p PersonSummary
	var given string
	err := s.DB.QueryRowContext(ctx, `
		SELECT p.id, COALESCE(n.surname,''), COALESCE(n.given_names,'[]'), p.sex, p.living
		FROM persons p LEFT JOIN names n ON n.person_id = p.id AND n.is_primary = 1
		WHERE p.id = ?`, id).Scan(&p.ID, &p.Surname, &given, &p.Sex, &p.Living)
	if errors.Is(err, sql.ErrNoRows) {
		return p, fmt.Errorf("personne %s: %w", id, ErrNotFound)
	}
	p.GivenNames = parseGiven(given)
	return p, err
}

// CountPersons renvoie le nombre de personnes.
func (s *Store) CountPersons(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM persons`).Scan(&n)
	return n, err
}

// --- Fiche : identité ------------------------------------------------------

type NameDTO struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Surname    string   `json:"surname"`
	GivenNames []string `json:"givenNames"`
	UsualGiven int      `json:"usualGiven"`
	Prefix     string   `json:"prefix"`
	Suffix     string   `json:"suffix"`
	IsPrimary  bool     `json:"isPrimary"`
}

type PersonCore struct {
	ID         string    `json:"id"`
	Sex        string    `json:"sex"`
	Living     string    `json:"living"`
	IsFavorite bool      `json:"isFavorite"`
	NotesMD    string    `json:"notesMd"`
	CreatedAt  string    `json:"createdAt"`
	UpdatedAt  string    `json:"updatedAt"`
	Names      []NameDTO `json:"names"`
}

func (s *Store) GetCore(ctx context.Context, id string) (PersonCore, error) {
	var c PersonCore
	var fav int
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, sex, living, is_favorite, notes_md, created_at, updated_at FROM persons WHERE id = ?`, id).
		Scan(&c.ID, &c.Sex, &c.Living, &fav, &c.NotesMD, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, fmt.Errorf("personne %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return c, err
	}
	c.IsFavorite = fav == 1
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, type, surname, given_names, usual_given, prefix, suffix, is_primary
		FROM names WHERE person_id = ? ORDER BY is_primary DESC, rowid`, id)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	c.Names = []NameDTO{}
	for rows.Next() {
		var n NameDTO
		var given string
		var prim int
		if err := rows.Scan(&n.ID, &n.Type, &n.Surname, &given, &n.UsualGiven, &n.Prefix, &n.Suffix, &prim); err != nil {
			return c, err
		}
		n.GivenNames, n.IsPrimary = parseGiven(given), prim == 1
		c.Names = append(c.Names, n)
	}
	return c, rows.Err()
}

// PersonPatch : champs modifiables (nil = inchangé).
type PersonPatch struct {
	Sex        *string `json:"sex"`
	Living     *string `json:"living"`
	IsFavorite *bool   `json:"isFavorite"`
	NotesMD    *string `json:"notesMd"`
}

func (s *Store) UpdatePerson(ctx context.Context, id string, p PersonPatch) error {
	fields := Row{}
	if p.Sex != nil {
		fields["sex"] = *p.Sex
	}
	if p.Living != nil {
		fields["living"] = *p.Living
	}
	if p.IsFavorite != nil {
		fields["is_favorite"] = boolInt(*p.IsFavorite)
	}
	if p.NotesMD != nil {
		fields["notes_md"] = *p.NotesMD
	}
	if len(fields) == 0 {
		return nil
	}
	_, err := s.Mutate(ctx, "Modifier une personne", func(t *Tx) error {
		fields["updated_at"] = t.Now()
		return t.Update("persons", id, fields)
	})
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// NameInput décrit un nom à créer ou modifier.
type NameInput struct {
	Type       string   `json:"type"`
	Surname    string   `json:"surname"`
	GivenNames []string `json:"givenNames"`
	UsualGiven int      `json:"usualGiven"`
	Prefix     string   `json:"prefix"`
	Suffix     string   `json:"suffix"`
}

func (n NameInput) row() Row {
	if n.Type == "" {
		n.Type = "other"
	}
	if n.GivenNames == nil {
		n.GivenNames = []string{}
	}
	g, _ := json.Marshal(n.GivenNames)
	return Row{"type": n.Type, "surname": n.Surname, "given_names": string(g),
		"usual_given": n.UsualGiven, "prefix": n.Prefix, "suffix": n.Suffix}
}

func (s *Store) AddName(ctx context.Context, personID string, n NameInput) (string, error) {
	id := NewID()
	_, err := s.Mutate(ctx, "Ajouter un nom", func(t *Tx) error {
		r := n.row()
		r["id"], r["person_id"], r["is_primary"] = id, personID, 0
		if err := t.Insert("names", r); err != nil {
			return err
		}
		return t.touchPerson(personID)
	})
	return id, err
}

func (s *Store) UpdateName(ctx context.Context, nameID string, n NameInput) error {
	_, err := s.Mutate(ctx, "Modifier un nom", func(t *Tx) error {
		return t.Update("names", nameID, n.row())
	})
	return err
}

// SetPrimaryName désigne le nom affiché par défaut (un seul par personne).
func (s *Store) SetPrimaryName(ctx context.Context, nameID string) error {
	_, err := s.Mutate(ctx, "Changer le nom principal", func(t *Tx) error {
		var pid string
		if err := t.tx.QueryRowContext(t.ctx, `SELECT person_id FROM names WHERE id = ?`, nameID).Scan(&pid); err != nil {
			return fmt.Errorf("nom %s: %w", nameID, ErrNotFound)
		}
		ids, err := t.dependentIDs(dependent{table: "names", column: "person_id"}, pid)
		if err != nil {
			return err
		}
		// l'ancien principal d'abord, pour respecter l'unicité
		for _, id := range ids {
			if id != nameID {
				if err := t.updateIfChanged("names", id, Row{"is_primary": 0}); err != nil {
					return err
				}
			}
		}
		return t.updateIfChanged("names", nameID, Row{"is_primary": 1})
	})
	return err
}

// updateIfChanged évite de journaliser une mise à jour sans effet.
func (t *Tx) updateIfChanged(table, id string, fields Row) error {
	cur, err := selectRow(t.ctx, t.tx, table, id)
	if err != nil || cur == nil {
		return err
	}
	same := true
	for k, v := range fields {
		if fmt.Sprint(cur[k]) != fmt.Sprint(v) {
			same = false
		}
	}
	if same {
		return nil
	}
	return t.Update(table, id, fields)
}

// DeleteName refuse de supprimer le nom principal ou le dernier nom.
func (s *Store) DeleteName(ctx context.Context, nameID string) error {
	_, err := s.Mutate(ctx, "Supprimer un nom", func(t *Tx) error {
		var prim int
		if err := t.tx.QueryRowContext(t.ctx, `SELECT is_primary FROM names WHERE id = ?`, nameID).Scan(&prim); err != nil {
			return fmt.Errorf("nom %s: %w", nameID, ErrNotFound)
		}
		if prim == 1 {
			return errors.New("le nom principal ne peut pas être supprimé : désignez d'abord un autre nom principal")
		}
		return t.Delete("names", nameID)
	})
	return err
}

// --- Suppression -----------------------------------------------------------

// DeleteMode : portée d'une suppression (spec §5.2).
type DeleteMode string

const (
	DeleteOnly        DeleteMode = "only"        // la personne seule
	DeleteDescendants DeleteMode = "descendants" // + descendants exclusifs
	DeleteAncestors   DeleteMode = "ancestors"   // + ancêtres exclusifs
)

// Impact résume ce qu'une suppression retirerait.
type Impact struct {
	People           []PersonSummary `json:"people"` // personnes supprimées (la première est la cible)
	Events           int             `json:"events"`
	ParentLinks      int             `json:"parentLinks"`
	Unions           int             `json:"unions"`
	MediaLinks       int             `json:"mediaLinks"`
	Citations        int             `json:"citations"`
	BecomingIsolated []PersonSummary `json:"becomingIsolated"` // restantes, sans plus aucun lien
}

// deletionSet calcule les personnes à supprimer. Une personne liée est incluse
// seulement si TOUS ses liens dans la direction choisie mènent à l'ensemble
// (c'est-à-dire qu'elle n'a pas d'autre rattachement).
func (s *Store) deletionSet(ctx context.Context, q execer, id string, mode DeleteMode) ([]string, error) {
	set := map[string]bool{id: true}
	order := []string{id}
	if mode == DeleteOnly {
		return order, nil
	}
	// next : voisins candidats ; others : liens opposés qui doivent tous rester dans l'ensemble
	var nextQ, otherQ string
	if mode == DeleteDescendants {
		nextQ = `SELECT child_id FROM parent_child WHERE parent_id = ?`
		otherQ = `SELECT parent_id FROM parent_child WHERE child_id = ?`
	} else {
		nextQ = `SELECT parent_id FROM parent_child WHERE child_id = ?`
		otherQ = `SELECT child_id FROM parent_child WHERE parent_id = ?`
	}
	col := func(query, arg string) ([]string, error) {
		rows, err := q.QueryContext(ctx, query, arg)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, rows.Err()
	}
	for i := 0; i < len(order); i++ {
		next, err := col(nextQ, order[i])
		if err != nil {
			return nil, err
		}
		for _, n := range next {
			if set[n] {
				continue
			}
			others, err := col(otherQ, n)
			if err != nil {
				return nil, err
			}
			ok := true
			for _, o := range others {
				if !set[o] {
					ok = false
					break
				}
			}
			if ok {
				set[n] = true
				order = append(order, n)
			}
		}
	}
	return order, nil
}

// DeleteImpact calcule l'aperçu d'impact d'une suppression (sans rien modifier).
func (s *Store) DeleteImpact(ctx context.Context, id string, mode DeleteMode) (Impact, error) {
	var imp Impact
	ids, err := s.deletionSet(ctx, s.DB, id, mode)
	if err != nil {
		return imp, err
	}
	in := map[string]bool{}
	imp.People = []PersonSummary{}
	imp.BecomingIsolated = []PersonSummary{}
	for _, pid := range ids {
		sum, err := s.GetSummary(ctx, pid)
		if err != nil {
			return imp, err
		}
		in[pid] = true
		imp.People = append(imp.People, sum)
	}
	count := func(q string, id string) int {
		var n int
		_ = s.DB.QueryRowContext(ctx, q, id).Scan(&n)
		return n
	}
	linked := map[string]bool{}
	for _, pid := range ids {
		imp.Events += count(`SELECT COUNT(*) FROM events WHERE person_id = ?`, pid)
		imp.ParentLinks += count(`SELECT COUNT(*) FROM parent_child WHERE child_id = ?1 OR parent_id = ?1`, pid)
		imp.MediaLinks += count(`SELECT COUNT(*) FROM media_links WHERE target_type='person' AND target_id = ?`, pid)
		imp.Citations += count(`SELECT COUNT(*) FROM citations WHERE target_type='person' AND target_id = ?`, pid)
		rows, err := s.DB.QueryContext(ctx, `
			SELECT child_id FROM parent_child WHERE parent_id = ?1
			UNION SELECT parent_id FROM parent_child WHERE child_id = ?1
			UNION SELECT up2.person_id FROM union_partners up1 JOIN union_partners up2
			      ON up1.union_id = up2.union_id AND up2.person_id <> up1.person_id WHERE up1.person_id = ?1`, pid)
		if err != nil {
			return imp, err
		}
		for rows.Next() {
			var o string
			rows.Scan(&o)
			if !in[o] {
				linked[o] = true
			}
		}
		rows.Close()
	}
	imp.Unions = 0
	seenU := map[string]bool{}
	for _, pid := range ids {
		rows, err := s.DB.QueryContext(ctx, `SELECT union_id FROM union_partners WHERE person_id = ?`, pid)
		if err != nil {
			return imp, err
		}
		for rows.Next() {
			var u string
			rows.Scan(&u)
			seenU[u] = true
		}
		rows.Close()
	}
	imp.Unions = len(seenU)
	for o := range linked {
		if s.isolatedAfter(ctx, o, in) {
			sum, err := s.GetSummary(ctx, o)
			if err == nil {
				imp.BecomingIsolated = append(imp.BecomingIsolated, sum)
			}
		}
	}
	return imp, nil
}

// isolatedAfter : tous les liens de la personne vont vers des personnes supprimées.
func (s *Store) isolatedAfter(ctx context.Context, id string, deleted map[string]bool) bool {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT child_id FROM parent_child WHERE parent_id = ?1
		UNION SELECT parent_id FROM parent_child WHERE child_id = ?1
		UNION SELECT up2.person_id FROM union_partners up1 JOIN union_partners up2
		      ON up1.union_id = up2.union_id AND up2.person_id <> up1.person_id WHERE up1.person_id = ?1`, id)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var o string
		rows.Scan(&o)
		if !deleted[o] {
			return false
		}
	}
	return true
}

// DeletePerson supprime une personne seule (annulable).
func (s *Store) DeletePerson(ctx context.Context, id string) error {
	return s.DeletePeople(ctx, id, DeleteOnly)
}

// DeletePeople supprime la personne et, selon mode, ses descendants ou
// ancêtres exclusifs, en une seule entrée d'historique.
func (s *Store) DeletePeople(ctx context.Context, id string, mode DeleteMode) error {
	label := map[DeleteMode]string{
		DeleteOnly: "Supprimer une personne", DeleteDescendants: "Supprimer une branche descendante",
		DeleteAncestors: "Supprimer une branche ascendante",
	}[mode]
	if label == "" {
		return fmt.Errorf("mode de suppression inconnu: %q", mode)
	}
	_, err := s.Mutate(ctx, label, func(t *Tx) error {
		ids, err := s.deletionSet(ctx, t.tx, id, mode)
		if err != nil {
			return err
		}
		// unions touchées, pour retirer celles qui n'auront plus de partenaire
		touched := map[string]bool{}
		for _, pid := range ids {
			us, err := t.columnValues(`SELECT union_id FROM union_partners WHERE person_id = ?`, pid)
			if err != nil {
				return err
			}
			for _, u := range us {
				touched[u] = true
			}
		}
		for _, pid := range ids {
			if err := t.Delete("persons", pid); err != nil {
				return err
			}
		}
		for u := range touched {
			left, err := t.columnValues(`SELECT id FROM union_partners WHERE union_id = ?`, u)
			if err != nil {
				return err
			}
			if len(left) == 0 {
				if err := t.Delete("unions", u); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return err
}

func (t *Tx) columnValues(q string, args ...any) ([]string, error) {
	rows, err := t.tx.QueryContext(t.ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
