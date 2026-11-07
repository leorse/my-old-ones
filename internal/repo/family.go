package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrNoParents    = errors.New("cette personne n'a pas de parents : ajoutez d'abord un parent")
	ErrParentExists = errors.New("un parent biologique avec ce rôle existe déjà")
)

// --- Filiation -------------------------------------------------------------

// wouldCreateCycle : parentID descend-il déjà de childID ? (alors lier
// childID comme enfant de parentID fermerait une boucle).
func (t *Tx) wouldCreateCycle(childID, parentID string) (bool, error) {
	var n int
	err := t.tx.QueryRowContext(t.ctx, `
		WITH RECURSIVE descendants(id) AS (
			SELECT child_id FROM parent_child WHERE parent_id = ?1
			UNION
			SELECT pc.child_id FROM parent_child pc JOIN descendants d ON pc.parent_id = d.id
		)
		SELECT COUNT(*) FROM descendants WHERE id = ?2`, childID, parentID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("détection de cycle: %w", err)
	}
	return n > 0, nil
}

func (t *Tx) linkParent(childID, parentID, role, kind, unionID string, order any) (string, error) {
	if role == "" {
		role = "parent"
	}
	if kind == "" {
		kind = "biological"
	}
	if childID == parentID {
		return "", ErrSelfParent
	}
	cyc, err := t.wouldCreateCycle(childID, parentID)
	if err != nil {
		return "", err
	}
	if cyc {
		return "", ErrCycle
	}
	if kind == "biological" && (role == "father" || role == "mother") {
		n, err := t.columnValues(`SELECT id FROM parent_child WHERE child_id = ? AND parent_role = ? AND kind = 'biological'`, childID, role)
		if err != nil {
			return "", err
		}
		if len(n) > 0 {
			return "", ErrParentExists
		}
	}
	id := NewID()
	row := Row{"id": id, "child_id": childID, "parent_id": parentID, "parent_role": role, "kind": kind, "birth_order": order}
	if unionID != "" {
		row["union_id"] = unionID
	}
	return id, t.Insert("parent_child", row)
}

// LinkParent crée un lien de filiation en refusant auto-référence, cycles et
// second parent biologique de même rôle.
func (s *Store) LinkParent(ctx context.Context, childID, parentID, role, kind string) (string, error) {
	var id string
	_, err := s.Mutate(ctx, "Ajouter un lien de filiation", func(t *Tx) (err error) {
		id, err = t.linkParent(childID, parentID, role, kind, "", nil)
		return
	})
	return id, err
}

// UnlinkParent supprime le lien sans supprimer les personnes (« délier »).
func (s *Store) UnlinkParent(ctx context.Context, linkID string) error {
	_, err := s.Mutate(ctx, "Délier un parent", func(t *Tx) error { return t.Delete("parent_child", linkID) })
	return err
}

// ParentLinkPatch : champs modifiables d'un lien (nil = inchangé).
type ParentLinkPatch struct {
	Role    *string `json:"role"`
	Kind    *string `json:"kind"`
	UnionID *string `json:"unionId"` // "" retire l'union
}

func (s *Store) UpdateParentLink(ctx context.Context, linkID string, p ParentLinkPatch) error {
	fields := Row{}
	if p.Role != nil {
		fields["parent_role"] = *p.Role
	}
	if p.Kind != nil {
		fields["kind"] = *p.Kind
	}
	if p.UnionID != nil {
		if *p.UnionID == "" {
			fields["union_id"] = nil
		} else {
			fields["union_id"] = *p.UnionID
		}
	}
	if len(fields) == 0 {
		return nil
	}
	_, err := s.Mutate(ctx, "Modifier un lien de filiation", func(t *Tx) error {
		return t.Update("parent_child", linkID, fields)
	})
	return err
}

// ReorderChildren fixe l'ordre de fratrie à partir d'une liste de liens parent_child.
func (s *Store) ReorderChildren(ctx context.Context, linkIDs []string) error {
	_, err := s.Mutate(ctx, "Réordonner les enfants", func(t *Tx) error {
		for i, id := range linkIDs {
			if err := t.updateIfChanged("parent_child", id, Row{"birth_order": i + 1}); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// --- Unions ----------------------------------------------------------------

func (t *Tx) createUnion(typ string, partners ...string) (string, error) {
	if typ == "" {
		typ = "marriage"
	}
	id := NewID()
	if err := t.Insert("unions", Row{"id": id, "type": typ, "created_at": t.Now(), "updated_at": t.Now()}); err != nil {
		return "", err
	}
	for i, p := range partners {
		if err := t.Insert("union_partners", Row{"id": NewID(), "union_id": id, "person_id": p, "position": i}); err != nil {
			return "", err
		}
	}
	return id, nil
}

// CreateUnion crée une union entre des personnes existantes.
func (s *Store) CreateUnion(ctx context.Context, typ string, partners []string) (string, error) {
	var id string
	_, err := s.Mutate(ctx, "Créer une union", func(t *Tx) (err error) {
		if len(partners) == 2 && partners[0] == partners[1] {
			return errors.New("une union ne peut pas lier une personne à elle-même")
		}
		id, err = t.createUnion(typ, partners...)
		return
	})
	return id, err
}

type UnionPatch struct {
	Type    *string `json:"type"`
	Status  *string `json:"status"`
	NotesMD *string `json:"notesMd"`
}

func (s *Store) UpdateUnion(ctx context.Context, id string, p UnionPatch) error {
	fields := Row{}
	if p.Type != nil {
		fields["type"] = *p.Type
	}
	if p.Status != nil {
		fields["status"] = *p.Status
	}
	if p.NotesMD != nil {
		fields["notes_md"] = *p.NotesMD
	}
	if len(fields) == 0 {
		return nil
	}
	_, err := s.Mutate(ctx, "Modifier une union", func(t *Tx) error {
		fields["updated_at"] = t.Now()
		return t.Update("unions", id, fields)
	})
	return err
}

// DeleteUnion supprime l'union et ses événements ; les enfants restent liés à leurs parents.
func (s *Store) DeleteUnion(ctx context.Context, id string) error {
	_, err := s.Mutate(ctx, "Supprimer une union", func(t *Tx) error { return t.Delete("unions", id) })
	return err
}

// --- Relations rapides (spec §5.2) -----------------------------------------

type Relation string

const (
	RelFather  Relation = "father"
	RelMother  Relation = "mother"
	RelSpouse  Relation = "spouse"
	RelChild   Relation = "child"
	RelSibling Relation = "sibling"
)

// RelativeOpts précise la création d'un proche.
type RelativeOpts struct {
	UnionID string `json:"unionId"` // enfant : union des parents (sinon déduite si unique)
}

func roleForSex(sex string) string {
	switch sex {
	case "M":
		return "father"
	case "F":
		return "mother"
	}
	return "parent"
}

// CreateRelative crée un père, une mère, un conjoint, un enfant ou un
// frère/une sœur de personID, en une seule opération annulable.
func (s *Store) CreateRelative(ctx context.Context, personID string, rel Relation, np NewPerson, opts RelativeOpts) (string, error) {
	var newID string
	labels := map[Relation]string{RelFather: "Ajouter un père", RelMother: "Ajouter une mère",
		RelSpouse: "Ajouter un conjoint", RelChild: "Ajouter un enfant", RelSibling: "Ajouter un frère ou une sœur"}
	label, ok := labels[rel]
	if !ok {
		return "", fmt.Errorf("relation inconnue: %q", rel)
	}
	_, err := s.Mutate(ctx, label, func(t *Tx) error {
		var exists int
		if err := t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM persons WHERE id = ?`, personID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("personne %s: %w", personID, ErrNotFound)
		}
		switch rel {
		case RelFather, RelMother:
			role, sex := "father", "M"
			if rel == RelMother {
				role, sex = "mother", "F"
			}
			if np.Sex == "" {
				np.Sex = sex
			}
			dup, err := t.columnValues(`SELECT id FROM parent_child WHERE child_id = ? AND parent_role = ? AND kind = 'biological'`, personID, role)
			if err != nil {
				return err
			}
			if len(dup) > 0 {
				return ErrParentExists
			}
			id, err := t.createPerson(np)
			if err != nil {
				return err
			}
			newID = id
			if _, err := t.linkParent(personID, id, role, "biological", "", nil); err != nil {
				return err
			}
			// si l'autre parent existe déjà, relier les deux par une union
			other := "mother"
			if rel == RelMother {
				other = "father"
			}
			ops, err := t.columnValues(`SELECT parent_id FROM parent_child WHERE child_id = ? AND parent_role = ? AND kind='biological'`, personID, other)
			if err != nil {
				return err
			}
			if len(ops) == 1 {
				shared, err := t.columnValues(`SELECT a.union_id FROM union_partners a JOIN union_partners b
					ON a.union_id = b.union_id WHERE a.person_id = ? AND b.person_id = ?`, id, ops[0])
				if err != nil {
					return err
				}
				uid := ""
				if len(shared) > 0 {
					uid = shared[0]
				} else {
					partners := []string{id, ops[0]}
					if rel == RelMother {
						partners = []string{ops[0], id}
					}
					if uid, err = t.createUnion("marriage", partners...); err != nil {
						return err
					}
				}
				// rattacher les deux liens de filiation à l'union des parents
				links, err := t.columnValues(`SELECT id FROM parent_child WHERE child_id = ? AND parent_id IN (?, ?) AND union_id IS NULL`,
					personID, id, ops[0])
				if err != nil {
					return err
				}
				for _, l := range links {
					if err := t.Update("parent_child", l, Row{"union_id": uid}); err != nil {
						return err
					}
				}
			}
		case RelSpouse:
			id, err := t.createPerson(np)
			if err != nil {
				return err
			}
			newID = id
			if _, err := t.createUnion("marriage", personID, id); err != nil {
				return err
			}
		case RelChild:
			id, err := t.createPerson(np)
			if err != nil {
				return err
			}
			newID = id
			unionID := opts.UnionID
			if unionID == "" {
				us, err := t.columnValues(`SELECT union_id FROM union_partners WHERE person_id = ?`, personID)
				if err != nil {
					return err
				}
				if len(us) == 1 {
					unionID = us[0]
				}
			}
			parents := []string{personID}
			if unionID != "" {
				ps, err := t.columnValues(`SELECT person_id FROM union_partners WHERE union_id = ? ORDER BY position`, unionID)
				if err != nil {
					return err
				}
				parents = ps
				found := false
				for _, p := range ps {
					found = found || p == personID
				}
				if !found {
					return errors.New("cette union ne concerne pas la personne")
				}
			}
			for _, p := range parents {
				var sex string
				if err := t.tx.QueryRowContext(t.ctx, `SELECT sex FROM persons WHERE id = ?`, p).Scan(&sex); err != nil {
					return err
				}
				if _, err := t.linkParent(id, p, roleForSex(sex), "biological", unionID, nil); err != nil {
					return err
				}
			}
		case RelSibling:
			rows, err := t.tx.QueryContext(t.ctx,
				`SELECT parent_id, parent_role, kind, COALESCE(union_id,'') FROM parent_child WHERE child_id = ?`, personID)
			if err != nil {
				return err
			}
			type link struct{ parent, role, kind, union string }
			var links []link
			for rows.Next() {
				var l link
				if err := rows.Scan(&l.parent, &l.role, &l.kind, &l.union); err != nil {
					rows.Close()
					return err
				}
				links = append(links, l)
			}
			rows.Close()
			if len(links) == 0 {
				return ErrNoParents
			}
			id, err := t.createPerson(np)
			if err != nil {
				return err
			}
			newID = id
			for _, l := range links {
				if _, err := t.linkParent(id, l.parent, l.role, l.kind, l.union, nil); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return newID, nil
}

// --- Vue famille -----------------------------------------------------------

type ParentRef struct {
	LinkID string        `json:"linkId"`
	Role   string        `json:"role"`
	Kind   string        `json:"kind"`
	Person PersonSummary `json:"person"`
}

type ChildRef struct {
	LinkID string        `json:"linkId"`
	Kind   string        `json:"kind"`
	Person PersonSummary `json:"person"`
}

type UnionView struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Status   string          `json:"status"`
	Partners []PersonSummary `json:"partners"` // sans la personne consultée si possible
	Events   []EventDTO      `json:"events"`
	Children []ChildRef      `json:"children"`
}

type Family struct {
	Parents  []ParentRef     `json:"parents"`
	Unions   []UnionView     `json:"unions"`
	Siblings []PersonSummary `json:"siblings"`
	// Children hors union (parent unique ou union inconnue)
	OtherChildren []ChildRef `json:"otherChildren"`
}

func (s *Store) GetFamily(ctx context.Context, personID string) (Family, error) {
	f := Family{Parents: []ParentRef{}, Unions: []UnionView{}, Siblings: []PersonSummary{}, OtherChildren: []ChildRef{}}

	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, parent_id, parent_role, kind FROM parent_child WHERE child_id = ?
		 ORDER BY CASE parent_role WHEN 'father' THEN 0 WHEN 'mother' THEN 1 ELSE 2 END, rowid`, personID)
	if err != nil {
		return f, err
	}
	type pl struct{ link, parent, role, kind string }
	var pls []pl
	for rows.Next() {
		var x pl
		if err := rows.Scan(&x.link, &x.parent, &x.role, &x.kind); err != nil {
			rows.Close()
			return f, err
		}
		pls = append(pls, x)
	}
	rows.Close()
	for _, x := range pls {
		sum, err := s.GetSummary(ctx, x.parent)
		if err != nil {
			return f, err
		}
		f.Parents = append(f.Parents, ParentRef{LinkID: x.link, Role: x.role, Kind: x.kind, Person: sum})
	}

	// fratrie : enfants des mêmes parents, hors soi, sans doublon
	seen := map[string]bool{personID: true}
	for _, x := range pls {
		kids, err := s.childRefs(ctx, `SELECT id, child_id, kind FROM parent_child WHERE parent_id = ? ORDER BY COALESCE(birth_order, 1e9), rowid`, x.parent)
		if err != nil {
			return f, err
		}
		for _, k := range kids {
			if !seen[k.Person.ID] {
				seen[k.Person.ID] = true
				f.Siblings = append(f.Siblings, k.Person)
			}
		}
	}

	// unions
	urows, err := s.DB.QueryContext(ctx, `
		SELECT u.id, u.type, u.status FROM unions u JOIN union_partners up ON up.union_id = u.id
		WHERE up.person_id = ? ORDER BY up.rowid`, personID)
	if err != nil {
		return f, err
	}
	var uvs []UnionView
	for urows.Next() {
		var u UnionView
		if err := urows.Scan(&u.ID, &u.Type, &u.Status); err != nil {
			urows.Close()
			return f, err
		}
		uvs = append(uvs, u)
	}
	urows.Close()
	inUnion := map[string]bool{}
	for _, u := range uvs {
		u.Partners = []PersonSummary{}
		prs, err := s.DB.QueryContext(ctx, `SELECT person_id FROM union_partners WHERE union_id = ? AND person_id <> ? ORDER BY position`, u.ID, personID)
		if err != nil {
			return f, err
		}
		var pids []string
		for prs.Next() {
			var p string
			prs.Scan(&p)
			pids = append(pids, p)
		}
		prs.Close()
		for _, p := range pids {
			sum, err := s.GetSummary(ctx, p)
			if err != nil {
				return f, err
			}
			u.Partners = append(u.Partners, sum)
		}
		if u.Events, err = s.unionEvents(ctx, u.ID); err != nil {
			return f, err
		}
		u.Children, err = s.childRefs(ctx, `SELECT MIN(id), child_id, MIN(kind) FROM parent_child WHERE union_id = ? GROUP BY child_id ORDER BY COALESCE(MIN(birth_order), 1e9), MIN(rowid)`, u.ID)
		if err != nil {
			return f, err
		}
		for _, c := range u.Children {
			inUnion[c.Person.ID] = true
		}
		f.Unions = append(f.Unions, u)
	}
	kids, err := s.childRefs(ctx, `SELECT id, child_id, kind FROM parent_child WHERE parent_id = ? ORDER BY COALESCE(birth_order, 1e9), rowid`, personID)
	if err != nil {
		return f, err
	}
	for _, k := range kids {
		if !inUnion[k.Person.ID] {
			f.OtherChildren = append(f.OtherChildren, k)
		}
	}
	return f, nil
}

func (s *Store) childRefs(ctx context.Context, q string, arg string) ([]ChildRef, error) {
	rows, err := s.DB.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	type raw struct{ link, child, kind string }
	var raws []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.link, &r.child, &r.kind); err != nil {
			rows.Close()
			return nil, err
		}
		raws = append(raws, r)
	}
	rows.Close()
	out := []ChildRef{}
	for _, r := range raws {
		sum, err := s.GetSummary(ctx, r.child)
		if err != nil {
			if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		out = append(out, ChildRef{LinkID: r.link, Kind: r.kind, Person: sum})
	}
	return out, nil
}
