package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// MaxTreeNodes borne la taille d'un sous-arbre renvoyé au frontend.
const MaxTreeNodes = 3000

type TreeNode struct {
	Person PersonSummary `json:"person"`
	Birth  string        `json:"birth"` // année (grégorienne) ou ""
	Death  string        `json:"death"`
	// Présence de parents biologiques dans TOUTE la base (pas seulement dans le sous-arbre renvoyé) :
	// l'interface s'en sert pour proposer « ajouter un père / une mère ».
	HasFather  bool `json:"hasFather"`
	HasMother  bool `json:"hasMother"`
	HasParents bool `json:"hasParents"`
	// Nombre d'enfants (toutes natures de filiation) dans toute la base.
	ChildCount int `json:"childCount"`
}

type TreeLink struct {
	ID       string `json:"id"`
	ChildID  string `json:"childId"`
	ParentID string `json:"parentId"`
	Role     string `json:"role"`
	Kind     string `json:"kind"`
	UnionID  string `json:"unionId"`
}

type TreeUnion struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Status   string   `json:"status"`
	Partners []string `json:"partners"` // ordonnés, restreints aux nœuds renvoyés
	Year     string   `json:"year"`     // année du mariage si connue
}

// TreeData est le sous-graphe nécessaire pour dessiner un arbre autour d'une personne.
type TreeData struct {
	Focus     string      `json:"focus"`
	Nodes     []TreeNode  `json:"nodes"`
	Links     []TreeLink  `json:"links"`
	Unions    []TreeUnion `json:"unions"`
	Truncated bool        `json:"truncated"`
}

func jsonIDs(ids map[string]bool) string {
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	sort.Strings(list)
	b, _ := json.Marshal(list)
	return string(b)
}

// GetTree renvoie la personne focus, ses ancêtres sur `up` générations, ses
// descendants sur `down` générations, et les conjoints de la lignée descendante.
// Les liens de filiation et unions sont restreints aux personnes renvoyées.
func (s *Store) GetTree(ctx context.Context, focus string, up, down int) (TreeData, error) {
	td := TreeData{Focus: focus, Nodes: []TreeNode{}, Links: []TreeLink{}, Unions: []TreeUnion{}}
	if _, err := s.GetSummary(ctx, focus); err != nil {
		return td, err
	}
	if up < 0 {
		up = 0
	}
	if down < 0 {
		down = 0
	}

	// Graphe complet des filiations et unions (léger : deux colonnes d'identifiants).
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, child_id, parent_id, parent_role, kind, COALESCE(union_id,'') FROM parent_child ORDER BY rowid`)
	if err != nil {
		return td, err
	}
	var all []TreeLink
	parentsOf := map[string][]int{}
	childrenOf := map[string][]int{}
	for rows.Next() {
		var l TreeLink
		if err := rows.Scan(&l.ID, &l.ChildID, &l.ParentID, &l.Role, &l.Kind, &l.UnionID); err != nil {
			rows.Close()
			return td, err
		}
		all = append(all, l)
		parentsOf[l.ChildID] = append(parentsOf[l.ChildID], len(all)-1)
		childrenOf[l.ParentID] = append(childrenOf[l.ParentID], len(all)-1)
	}
	rows.Close()

	prow, err := s.DB.QueryContext(ctx, `SELECT union_id, person_id FROM union_partners ORDER BY union_id, position, rowid`)
	if err != nil {
		return td, err
	}
	partnersOf := map[string][]string{} // union -> personnes
	unionsOf := map[string][]string{}   // personne -> unions
	for prow.Next() {
		var u, p string
		if err := prow.Scan(&u, &p); err != nil {
			prow.Close()
			return td, err
		}
		partnersOf[u] = append(partnersOf[u], p)
		unionsOf[p] = append(unionsOf[p], u)
	}
	prow.Close()

	nodes := map[string]bool{focus: true}
	// ancêtres
	frontier := []string{focus}
	for g := 0; g < up && len(frontier) > 0; g++ {
		var next []string
		for _, p := range frontier {
			for _, i := range parentsOf[p] {
				if pid := all[i].ParentID; !nodes[pid] {
					nodes[pid] = true
					next = append(next, pid)
				}
			}
		}
		frontier = next
	}
	// descendants (+ conjoints à chaque niveau)
	descendants := map[string]bool{focus: true}
	frontier = []string{focus}
	for g := 0; ; g++ {
		for _, p := range frontier {
			for _, u := range unionsOf[p] {
				for _, sp := range partnersOf[u] {
					nodes[sp] = true
				}
			}
		}
		if g >= down {
			break
		}
		var next []string
		for _, p := range frontier {
			for _, i := range childrenOf[p] {
				if cid := all[i].ChildID; !descendants[cid] {
					descendants[cid] = true
					nodes[cid] = true
					next = append(next, cid)
				}
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = next
	}
	if len(nodes) > MaxTreeNodes {
		td.Truncated = true
		return td, nil
	}

	// liens et unions restreints aux nœuds
	for _, l := range all {
		if nodes[l.ChildID] && nodes[l.ParentID] {
			td.Links = append(td.Links, l)
		}
	}
	unionIDs := map[string]bool{}
	for p := range nodes {
		for _, u := range unionsOf[p] {
			unionIDs[u] = true
		}
	}
	// années de mariage + type/statut
	urows, err := s.DB.QueryContext(ctx, `
		SELECT u.id, u.type, u.status,
		       COALESCE((SELECT e.sort_key / 10000 FROM events e WHERE e.union_id = u.id AND e.type = 'marriage'
		                 AND e.sort_key IS NOT NULL ORDER BY e.sort_key LIMIT 1), 0)
		FROM unions u WHERE u.id IN (SELECT value FROM json_each(?))`, jsonIDs(unionIDs))
	if err != nil {
		return td, err
	}
	for urows.Next() {
		var u TreeUnion
		var year int
		if err := urows.Scan(&u.ID, &u.Type, &u.Status, &year); err != nil {
			urows.Close()
			return td, err
		}
		if year > 0 {
			u.Year = fmt.Sprint(year)
		}
		for _, p := range partnersOf[u.ID] {
			if nodes[p] {
				u.Partners = append(u.Partners, p)
			}
		}
		td.Unions = append(td.Unions, u)
	}
	urows.Close()
	sort.Slice(td.Unions, func(i, j int) bool { return td.Unions[i].ID < td.Unions[j].ID })

	// personnes + années de naissance / décès
	ids := jsonIDs(nodes)
	prs, err := s.DB.QueryContext(ctx, `
		SELECT p.id, COALESCE(n.surname,''), COALESCE(n.given_names,'[]'), p.sex, p.living
		FROM persons p LEFT JOIN names n ON n.person_id = p.id AND n.is_primary = 1
		WHERE p.id IN (SELECT value FROM json_each(?))
		ORDER BY n.surname COLLATE NOCASE, n.given_names COLLATE NOCASE`, ids)
	if err != nil {
		return td, err
	}
	index := map[string]int{}
	for prs.Next() {
		var n TreeNode
		var given string
		if err := prs.Scan(&n.Person.ID, &n.Person.Surname, &given, &n.Person.Sex, &n.Person.Living); err != nil {
			prs.Close()
			return td, err
		}
		n.Person.GivenNames = parseGiven(given)
		kids := map[string]bool{}
		for _, i := range childrenOf[n.Person.ID] {
			kids[all[i].ChildID] = true
		}
		n.ChildCount = len(kids)
		for _, i := range parentsOf[n.Person.ID] {
			n.HasParents = true
			if all[i].Kind == "biological" {
				n.HasFather = n.HasFather || all[i].Role == "father"
				n.HasMother = n.HasMother || all[i].Role == "mother"
			}
		}
		index[n.Person.ID] = len(td.Nodes)
		td.Nodes = append(td.Nodes, n)
	}
	prs.Close()

	ev, err := s.DB.QueryContext(ctx, `
		SELECT person_id, type, MIN(sort_key) / 10000 FROM events
		WHERE person_id IN (SELECT value FROM json_each(?)) AND type IN ('birth','death') AND sort_key IS NOT NULL
		GROUP BY person_id, type`, ids)
	if err != nil {
		return td, err
	}
	for ev.Next() {
		var pid, typ string
		var year int
		if err := ev.Scan(&pid, &typ, &year); err != nil {
			ev.Close()
			return td, err
		}
		if i, ok := index[pid]; ok {
			if typ == "birth" {
				td.Nodes[i].Birth = fmt.Sprint(year)
			} else {
				td.Nodes[i].Death = fmt.Sprint(year)
			}
		}
	}
	ev.Close()
	return td, nil
}
