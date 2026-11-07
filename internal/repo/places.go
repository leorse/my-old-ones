package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type PlaceDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	ParentID    string   `json:"parentId"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	CodeINSEE   string   `json:"codeInsee"`
	ValidFrom   string   `json:"validFrom"`
	ValidTo     string   `json:"validTo"`
	DisplayName string   `json:"displayName"`
}

type PlaceInput struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	ParentID  string   `json:"parentId"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	CodeINSEE string   `json:"codeInsee"`
	ValidFrom string   `json:"validFrom"`
	ValidTo   string   `json:"validTo"`
}

func (in PlaceInput) row() Row {
	typ := in.Type
	if typ == "" {
		typ = "commune"
	}
	var lat, lon any
	if in.Latitude != nil {
		lat = *in.Latitude
	}
	if in.Longitude != nil {
		lon = *in.Longitude
	}
	return Row{"name": strings.TrimSpace(in.Name), "type": typ, "parent_id": nullStr(in.ParentID),
		"latitude": lat, "longitude": lon, "code_insee": nullStr(in.CodeINSEE),
		"valid_from": nullStr(in.ValidFrom), "valid_to": nullStr(in.ValidTo)}
}

// recomputeDisplay recalcule display_name d'un lieu puis de ses descendants.
func (t *Tx) recomputeDisplay(id string, depth int) error {
	if depth > 20 {
		return errors.New("hiérarchie de lieux trop profonde ou cyclique")
	}
	var name string
	var parent sql.NullString
	if err := t.tx.QueryRowContext(t.ctx, `SELECT name, parent_id FROM places WHERE id = ?`, id).Scan(&name, &parent); err != nil {
		return err
	}
	display := name
	if parent.Valid {
		var pd string
		if err := t.tx.QueryRowContext(t.ctx, `SELECT display_name FROM places WHERE id = ?`, parent.String).Scan(&pd); err != nil {
			return err
		}
		if pd != "" {
			display = name + ", " + pd
		}
	}
	if err := t.updateIfChanged("places", id, Row{"display_name": display}); err != nil {
		return err
	}
	kids, err := t.columnValues(`SELECT id FROM places WHERE parent_id = ?`, id)
	if err != nil {
		return err
	}
	for _, k := range kids {
		if err := t.recomputeDisplay(k, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// isAncestorPlace : candidate est-il id lui-même ou l'un de ses descendants ?
func (t *Tx) placeInSubtree(root, candidate string) (bool, error) {
	var n int
	err := t.tx.QueryRowContext(t.ctx, `
		WITH RECURSIVE sub(id) AS (SELECT ?1 UNION SELECT p.id FROM places p JOIN sub s ON p.parent_id = s.id)
		SELECT COUNT(*) FROM sub WHERE id = ?2`, root, candidate).Scan(&n)
	return n > 0, err
}

func (s *Store) CreatePlace(ctx context.Context, in PlaceInput) (string, error) {
	if strings.TrimSpace(in.Name) == "" {
		return "", errors.New("le nom du lieu est obligatoire")
	}
	id := NewID()
	_, err := s.Mutate(ctx, "Créer un lieu", func(t *Tx) error {
		r := in.row()
		r["id"], r["display_name"] = id, in.Name
		if err := t.Insert("places", r); err != nil {
			return err
		}
		return t.recomputeDisplay(id, 0)
	})
	return id, err
}

func (s *Store) UpdatePlace(ctx context.Context, id string, in PlaceInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("le nom du lieu est obligatoire")
	}
	_, err := s.Mutate(ctx, "Modifier un lieu", func(t *Tx) error {
		if in.ParentID != "" {
			bad, err := t.placeInSubtree(id, in.ParentID)
			if err != nil {
				return err
			}
			if bad {
				return errors.New("un lieu ne peut pas être contenu dans lui-même")
			}
		}
		if err := t.Update("places", id, in.row()); err != nil {
			return err
		}
		return t.recomputeDisplay(id, 0)
	})
	return err
}

// DeletePlace supprime un lieu : événements et sous-lieux sont conservés, sans ce lieu.
func (s *Store) DeletePlace(ctx context.Context, id string) error {
	_, err := s.Mutate(ctx, "Supprimer un lieu", func(t *Tx) error {
		kids, err := t.columnValues(`SELECT id FROM places WHERE parent_id = ?`, id)
		if err != nil {
			return err
		}
		if err := t.Delete("places", id); err != nil {
			return err
		}
		for _, k := range kids {
			if err := t.recomputeDisplay(k, 0); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// ListPlaces renvoie les lieux dont le nom contient query (tous si vide), limités à limit.
func (s *Store) ListPlaces(ctx context.Context, query string, limit int) ([]PlaceDTO, error) {
	if limit <= 0 {
		limit = 200
	}
	like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query) + "%"
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, type, COALESCE(parent_id,''), latitude, longitude, COALESCE(code_insee,''),
		       COALESCE(valid_from,''), COALESCE(valid_to,''), display_name
		FROM places WHERE display_name LIKE ? ESCAPE '\' COLLATE NOCASE
		ORDER BY display_name COLLATE NOCASE LIMIT ?`, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlaceDTO{}
	for rows.Next() {
		var p PlaceDTO
		var lat, lon sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.ParentID, &lat, &lon, &p.CodeINSEE, &p.ValidFrom, &p.ValidTo, &p.DisplayName); err != nil {
			return nil, err
		}
		if lat.Valid {
			p.Latitude = &lat.Float64
		}
		if lon.Valid {
			p.Longitude = &lon.Float64
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

var _ = fmt.Sprintf
