package repo

import (
	"context"

	"moo/internal/domain"
)

const settingRoot = "root_person_id"

// SetRoot définit la personne de référence (Sosa, parentés, vues par défaut).
func (s *Store) SetRoot(ctx context.Context, personID string) error {
	if _, err := s.GetSummary(ctx, personID); err != nil {
		return err
	}
	return s.SetSetting(ctx, settingRoot, personID)
}

// Root renvoie l'identifiant de la personne racine ("" si non définie ou supprimée).
func (s *Store) Root(ctx context.Context) (string, error) {
	id, err := s.GetSetting(ctx, settingRoot, "")
	if err != nil || id == "" {
		return "", err
	}
	if _, err := s.GetSummary(ctx, id); err != nil {
		return "", nil
	}
	return id, nil
}

// SosaNumbers calcule dynamiquement les numéros Sosa depuis la racine (non stockés).
// Seuls les liens biologiques (ou de nature inconnue) comptent.
func (s *Store) SosaNumbers(ctx context.Context, root string, maxGen int) (map[string][]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT child_id, parent_id, parent_role FROM parent_child
		WHERE kind IN ('biological','unknown') AND parent_role IN ('father','mother')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parents := map[string]domain.Parents{}
	for rows.Next() {
		var child, parent, role string
		if err := rows.Scan(&child, &parent, &role); err != nil {
			return nil, err
		}
		p := parents[child]
		if role == "father" {
			p.Father = parent
		} else {
			p.Mother = parent
		}
		parents[child] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return domain.SosaNumbers(root, func(id string) domain.Parents { return parents[id] }, maxGen), nil
}
