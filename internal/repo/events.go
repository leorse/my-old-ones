package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"moo/internal/domain"
)

type EventDTO struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	PersonID    string      `json:"personId"`
	UnionID     string      `json:"unionId"`
	Date        domain.Date `json:"date"`
	DateDisplay string      `json:"dateDisplay"`
	PlaceID     string      `json:"placeId"`
	PlaceName   string      `json:"placeName"`
	Description string      `json:"description"`
	NotesMD     string      `json:"notesMd"`
}

// EventInput : la date est saisie en texte libre et interprétée côté Go.
type EventInput struct {
	Type        string `json:"type"`
	PersonID    string `json:"personId"` // exactement l'un de PersonID / UnionID à la création
	UnionID     string `json:"unionId"`
	Date        string `json:"date"`
	PlaceID     string `json:"placeId"`
	Description string `json:"description"`
	NotesMD     string `json:"notesMd"`
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func dateRow(d domain.Date) Row {
	if d.Precision == "" {
		d.Precision = domain.PrecNone
	}
	if d.Calendar == "" {
		d.Calendar = domain.CalGregorian
	}
	var key any
	if k, ok := d.SortKey(); ok {
		key = k
	}
	return Row{
		"date_precision": string(d.Precision), "date_calendar": string(d.Calendar),
		"y1": nullInt(d.Y1), "m1": nullInt(d.M1), "d1": nullInt(d.D1),
		"y2": nullInt(d.Y2), "m2": nullInt(d.M2), "d2": nullInt(d.D2),
		"sort_key": key, "original_text": d.OriginalText,
	}
}

const eventSelect = `SELECT e.id, e.type, COALESCE(e.person_id,''), COALESCE(e.union_id,''),
	e.date_precision, e.date_calendar, e.y1, e.m1, e.d1, e.y2, e.m2, e.d2, e.original_text,
	COALESCE(e.place_id,''), COALESCE(pl.display_name,''), e.description, e.notes_md
	FROM events e LEFT JOIN places pl ON pl.id = e.place_id `

func (s *Store) dateStyle(ctx context.Context) domain.DateStyle {
	v, _ := s.GetSetting(ctx, "date_style", string(domain.StyleLong))
	return domain.DateStyle(v)
}

func (s *Store) scanEvents(ctx context.Context, rows *sql.Rows) ([]EventDTO, error) {
	defer rows.Close()
	style := s.dateStyle(ctx)
	out := []EventDTO{}
	for rows.Next() {
		var e EventDTO
		var prec, cal string
		var y1, m1, d1, y2, m2, d2 sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Type, &e.PersonID, &e.UnionID, &prec, &cal, &y1, &m1, &d1, &y2, &m2, &d2,
			&e.Date.OriginalText, &e.PlaceID, &e.PlaceName, &e.Description, &e.NotesMD); err != nil {
			return nil, err
		}
		e.Date.Precision, e.Date.Calendar = domain.Precision(prec), domain.Calendar(cal)
		e.Date.Y1, e.Date.M1, e.Date.D1 = int(y1.Int64), int(m1.Int64), int(d1.Int64)
		e.Date.Y2, e.Date.M2, e.Date.D2 = int(y2.Int64), int(m2.Int64), int(d2.Int64)
		e.DateDisplay = e.Date.Format(style)
		out = append(out, e)
	}
	return out, rows.Err()
}

const eventOrder = ` ORDER BY e.sort_key IS NULL, e.sort_key, e.rowid`

func (s *Store) PersonEvents(ctx context.Context, personID string) ([]EventDTO, error) {
	rows, err := s.DB.QueryContext(ctx, eventSelect+`WHERE e.person_id = ?`+eventOrder, personID)
	if err != nil {
		return nil, err
	}
	return s.scanEvents(ctx, rows)
}

func (s *Store) unionEvents(ctx context.Context, unionID string) ([]EventDTO, error) {
	rows, err := s.DB.QueryContext(ctx, eventSelect+`WHERE e.union_id = ?`+eventOrder, unionID)
	if err != nil {
		return nil, err
	}
	return s.scanEvents(ctx, rows)
}

// syncLiving déduit le statut vivant/décédé : un événement de décès => décédé ;
// à la suppression du dernier décès, retour à « inconnu ».
func (t *Tx) syncLiving(personID string) error {
	n, err := t.columnValues(`SELECT id FROM events WHERE person_id = ? AND type = 'death'`, personID)
	if err != nil {
		return err
	}
	if len(n) > 0 {
		return t.updateIfChanged("persons", personID, Row{"living": "deceased"})
	}
	var living string
	if err := t.tx.QueryRowContext(t.ctx, `SELECT living FROM persons WHERE id = ?`, personID).Scan(&living); err != nil {
		return err
	}
	if living == "deceased" {
		return t.updateIfChanged("persons", personID, Row{"living": "unknown"})
	}
	return nil
}

func validateEvent(typ, personID, unionID string) error {
	if (personID == "") == (unionID == "") {
		return errors.New("un événement se rattache à une personne ou à une union, pas aux deux")
	}
	if personID != "" && !domain.IsPersonEventType(typ) {
		return fmt.Errorf("type d'événement invalide pour une personne: %q", typ)
	}
	if unionID != "" && !domain.IsUnionEventType(typ) {
		return fmt.Errorf("type d'événement invalide pour une union: %q", typ)
	}
	return nil
}

func (s *Store) AddEvent(ctx context.Context, in EventInput) (string, error) {
	if err := validateEvent(in.Type, in.PersonID, in.UnionID); err != nil {
		return "", err
	}
	d := domain.ParseDate(in.Date)
	id := NewID()
	_, err := s.Mutate(ctx, "Ajouter un événement", func(t *Tx) error {
		row := dateRow(d)
		row["id"], row["type"] = id, in.Type
		row["person_id"], row["union_id"] = nullStr(in.PersonID), nullStr(in.UnionID)
		row["place_id"], row["description"], row["notes_md"] = nullStr(in.PlaceID), in.Description, in.NotesMD
		if err := t.Insert("events", row); err != nil {
			return err
		}
		if in.PersonID != "" {
			if err := t.syncLiving(in.PersonID); err != nil {
				return err
			}
			return t.touchPerson(in.PersonID)
		}
		return nil
	})
	return id, err
}

// EventPatch : champs modifiables (nil = inchangé). Le rattachement ne change pas.
type EventPatch struct {
	Type        *string `json:"type"`
	Date        *string `json:"date"`
	PlaceID     *string `json:"placeId"` // "" retire le lieu
	Description *string `json:"description"`
	NotesMD     *string `json:"notesMd"`
}

func (s *Store) UpdateEvent(ctx context.Context, id string, p EventPatch) error {
	_, err := s.Mutate(ctx, "Modifier un événement", func(t *Tx) error {
		cur, err := selectRow(t.ctx, t.tx, "events", id)
		if err != nil {
			return err
		}
		if cur == nil {
			return fmt.Errorf("événement %s: %w", id, ErrNotFound)
		}
		pid, _ := cur["person_id"].(string)
		uid, _ := cur["union_id"].(string)
		fields := Row{}
		if p.Type != nil {
			if err := validateEvent(*p.Type, pid, uid); err != nil {
				return err
			}
			fields["type"] = *p.Type
		}
		if p.Date != nil {
			for k, v := range dateRow(domain.ParseDate(*p.Date)) {
				fields[k] = v
			}
		}
		if p.PlaceID != nil {
			fields["place_id"] = nullStr(*p.PlaceID)
		}
		if p.Description != nil {
			fields["description"] = *p.Description
		}
		if p.NotesMD != nil {
			fields["notes_md"] = *p.NotesMD
		}
		if len(fields) == 0 {
			return nil
		}
		if err := t.Update("events", id, fields); err != nil {
			return err
		}
		if pid != "" {
			return t.syncLiving(pid)
		}
		return nil
	})
	return err
}

func (s *Store) DeleteEvent(ctx context.Context, id string) error {
	_, err := s.Mutate(ctx, "Supprimer un événement", func(t *Tx) error {
		cur, err := selectRow(t.ctx, t.tx, "events", id)
		if err != nil || cur == nil {
			return err
		}
		if err := t.Delete("events", id); err != nil {
			return err
		}
		if pid, _ := cur["person_id"].(string); pid != "" {
			return t.syncLiving(pid)
		}
		return nil
	})
	return err
}
