// Package repo : accès SQLite, transactions et historique annuler/rétablir.
//
// Toute écriture métier passe par Store.Mutate : les lignes modifiées sont
// enregistrées (avant/après) dans la même transaction, ce qui rend chaque
// opération atomique et annulable (spec §3.2).
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Row est une ligne de table (nom de colonne -> valeur).
type Row map[string]any

// DefaultHistoryLimit : nombre d'opérations conservées (spec §3.2).
const DefaultHistoryLimit = 500

var ErrNothingToUndo = errors.New("rien à annuler")
var ErrNothingToRedo = errors.New("rien à rétablir")
var ErrNotFound = errors.New("introuvable")

var identRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Store encapsule la base d'un projet.
type Store struct {
	DB    *sql.DB
	Limit int
	Now   func() time.Time
}

func NewStore(db *sql.DB) *Store {
	return &Store{DB: db, Limit: DefaultHistoryLimit, Now: time.Now}
}

type change struct {
	table, id, op string
	before, after Row
}

// Tx est la vue transactionnelle fournie aux mutations.
type Tx struct {
	tx      *sql.Tx
	ctx     context.Context
	changes []change
	now     time.Time
}

// SQL expose la transaction pour les lectures (ne pas écrire par ce biais :
// les écritures non passées par Insert/Update/Delete ne sont pas annulables).
func (t *Tx) SQL() *sql.Tx { return t.tx }

// Ctx renvoie le contexte de l'opération.
func (t *Tx) Ctx() context.Context { return t.ctx }

// Now renvoie l'horodatage fixe de l'opération (RFC 3339, UTC).
func (t *Tx) Now() string { return t.now.UTC().Format(time.RFC3339) }

func checkTable(table string) error {
	if !identRe.MatchString(table) || !tracked[table] {
		return fmt.Errorf("table non suivie par l'historique: %q", table)
	}
	return nil
}

func sortedCols(r Row) []string {
	cols := make([]string, 0, len(r))
	for c := range r {
		cols = append(cols, c)
	}
	// ordre stable pour des requêtes reproductibles
	for i := 1; i < len(cols); i++ {
		for j := i; j > 0 && cols[j] < cols[j-1]; j-- {
			cols[j], cols[j-1] = cols[j-1], cols[j]
		}
	}
	return cols
}

type execer interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

func insertRow(ctx context.Context, x execer, table string, r Row) error {
	cols := sortedCols(r)
	args := make([]any, len(cols))
	for i, c := range cols {
		if !identRe.MatchString(c) {
			return fmt.Errorf("colonne invalide: %q", c)
		}
		args[i] = r[c]
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table,
		strings.Join(cols, ","), strings.TrimSuffix(strings.Repeat("?,", len(cols)), ","))
	_, err := x.ExecContext(ctx, q, args...)
	return err
}

func updateRow(ctx context.Context, x execer, table, id string, r Row) error {
	var sets []string
	var args []any
	for _, c := range sortedCols(r) {
		if c == "id" {
			continue
		}
		if !identRe.MatchString(c) {
			return fmt.Errorf("colonne invalide: %q", c)
		}
		sets = append(sets, c+" = ?")
		args = append(args, r[c])
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	res, err := x.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", table, strings.Join(sets, ", ")), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%s/%s: %w", table, id, ErrNotFound)
	}
	return nil
}

func deleteRow(ctx context.Context, x execer, table, id string) error {
	_, err := x.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", table), id)
	return err
}

func selectRow(ctx context.Context, x execer, table, id string) (Row, error) {
	rows, err := x.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE id = ?", table), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, rows.Err()
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	r := make(Row, len(cols))
	for i, c := range cols {
		if b, ok := vals[i].([]byte); ok {
			vals[i] = string(b)
		}
		r[c] = vals[i]
	}
	return r, nil
}

// Insert ajoute une ligne ; r["id"] est obligatoire.
func (t *Tx) Insert(table string, r Row) error {
	if err := checkTable(table); err != nil {
		return err
	}
	id, _ := r["id"].(string)
	if id == "" {
		return fmt.Errorf("insert %s: id manquant", table)
	}
	if err := insertRow(t.ctx, t.tx, table, r); err != nil {
		return fmt.Errorf("insert %s: %w", table, err)
	}
	after, err := selectRow(t.ctx, t.tx, table, id)
	if err != nil {
		return err
	}
	t.changes = append(t.changes, change{table, id, "insert", nil, after})
	return nil
}

// Update modifie les colonnes données d'une ligne existante.
func (t *Tx) Update(table, id string, fields Row) error {
	if err := checkTable(table); err != nil {
		return err
	}
	before, err := selectRow(t.ctx, t.tx, table, id)
	if err != nil {
		return err
	}
	if before == nil {
		return fmt.Errorf("%s/%s: %w", table, id, ErrNotFound)
	}
	if err := updateRow(t.ctx, t.tx, table, id, fields); err != nil {
		return fmt.Errorf("update %s: %w", table, err)
	}
	after, err := selectRow(t.ctx, t.tx, table, id)
	if err != nil {
		return err
	}
	t.changes = append(t.changes, change{table, id, "update", before, after})
	return nil
}

// Delete supprime une ligne ET ses dépendants (voir dependents), chaque
// suppression étant journalisée, enfants d'abord.
func (t *Tx) Delete(table, id string) error {
	if err := checkTable(table); err != nil {
		return err
	}
	for _, dep := range dependents[table] {
		ids, err := t.dependentIDs(dep, id)
		if err != nil {
			return err
		}
		for _, cid := range ids {
			if err := t.Delete(dep.table, cid); err != nil {
				return err
			}
		}
	}
	for _, n := range nullifiers[table] {
		ids, err := t.dependentIDs(dependent{table: n.table, column: n.column}, id)
		if err != nil {
			return err
		}
		for _, rid := range ids {
			if err := t.Update(n.table, rid, Row{n.column: nil}); err != nil {
				return err
			}
		}
	}
	before, err := selectRow(t.ctx, t.tx, table, id)
	if err != nil {
		return err
	}
	if before == nil {
		return nil
	}
	if err := deleteRow(t.ctx, t.tx, table, id); err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	t.changes = append(t.changes, change{table, id, "delete", before, nil})
	return nil
}

func (t *Tx) dependentIDs(d dependent, parentID string) ([]string, error) {
	q := fmt.Sprintf("SELECT id FROM %s WHERE %s = ?", d.table, d.column)
	args := []any{parentID}
	if d.typeColumn != "" {
		q += fmt.Sprintf(" AND %s = ?", d.typeColumn)
		args = append(args, d.typeValue)
	}
	rows, err := t.tx.QueryContext(t.ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Mutate exécute fn dans une transaction et journalise une entrée d'historique
// unique (atomique, spec §3.2). Renvoie l'id de l'entrée, 0 si rien n'a changé.
func (s *Store) Mutate(ctx context.Context, label string, fn func(*Tx) error) (int64, error) {
	sqlTx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer sqlTx.Rollback()

	t := &Tx{tx: sqlTx, ctx: ctx, now: s.Now()}
	if err := fn(t); err != nil {
		return 0, err
	}
	if len(t.changes) == 0 {
		return 0, nil
	}

	// Une nouvelle opération invalide la pile « rétablir ».
	if _, err := sqlTx.ExecContext(ctx, `DELETE FROM history_entries WHERE undone = 1`); err != nil {
		return 0, err
	}
	res, err := sqlTx.ExecContext(ctx, `INSERT INTO history_entries(label, created_at) VALUES (?,?)`, label, t.Now())
	if err != nil {
		return 0, err
	}
	entryID, _ := res.LastInsertId()
	for i, c := range t.changes {
		b, err := marshalRow(c.before)
		if err != nil {
			return 0, err
		}
		a, err := marshalRow(c.after)
		if err != nil {
			return 0, err
		}
		if _, err := sqlTx.ExecContext(ctx,
			`INSERT INTO history_changes(entry_id, seq, tbl, row_id, op, before, after) VALUES (?,?,?,?,?,?,?)`,
			entryID, i, c.table, c.id, c.op, b, a); err != nil {
			return 0, err
		}
	}
	if s.Limit > 0 {
		if _, err := sqlTx.ExecContext(ctx, `DELETE FROM history_entries WHERE id <=
			(SELECT id FROM history_entries ORDER BY id DESC LIMIT 1 OFFSET ?)`, s.Limit); err != nil {
			return 0, err
		}
	}
	return entryID, sqlTx.Commit()
}

func marshalRow(r Row) (any, error) {
	if r == nil {
		return nil, nil
	}
	b, err := json.Marshal(r)
	return string(b), err
}

func unmarshalRow(s sql.NullString) (Row, error) {
	if !s.Valid {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(s.String))
	dec.UseNumber()
	var r Row
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	for k, v := range r {
		if n, ok := v.(json.Number); ok {
			if i, err := n.Int64(); err == nil {
				r[k] = i
			} else if f, err := n.Float64(); err == nil {
				r[k] = f
			}
		}
	}
	return r, nil
}

// HistoryState décrit l'état des piles annuler / rétablir.
type HistoryState struct {
	CanUndo   bool   `json:"canUndo"`
	CanRedo   bool   `json:"canRedo"`
	UndoLabel string `json:"undoLabel"`
	RedoLabel string `json:"redoLabel"`
}

func (s *Store) HistoryState(ctx context.Context) (HistoryState, error) {
	var h HistoryState
	err := s.DB.QueryRowContext(ctx,
		`SELECT label FROM history_entries WHERE undone = 0 ORDER BY id DESC LIMIT 1`).Scan(&h.UndoLabel)
	if err == nil {
		h.CanUndo = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return h, err
	}
	err = s.DB.QueryRowContext(ctx,
		`SELECT label FROM history_entries WHERE undone = 1 ORDER BY id ASC LIMIT 1`).Scan(&h.RedoLabel)
	if err == nil {
		h.CanRedo = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return h, err
	}
	return h, nil
}

func (s *Store) loadChanges(ctx context.Context, x execer, entryID int64) ([]change, error) {
	rows, err := x.QueryContext(ctx,
		`SELECT tbl, row_id, op, before, after FROM history_changes WHERE entry_id = ? ORDER BY seq`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []change
	for rows.Next() {
		var c change
		var b, a sql.NullString
		if err := rows.Scan(&c.table, &c.id, &c.op, &b, &a); err != nil {
			return nil, err
		}
		if c.before, err = unmarshalRow(b); err != nil {
			return nil, err
		}
		if c.after, err = unmarshalRow(a); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Undo annule la dernière opération. Renvoie son libellé.
func (s *Store) Undo(ctx context.Context) (string, error) {
	return s.replay(ctx, true)
}

// Redo rétablit la dernière opération annulée. Renvoie son libellé.
func (s *Store) Redo(ctx context.Context) (string, error) {
	return s.replay(ctx, false)
}

func (s *Store) replay(ctx context.Context, undo bool) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var id int64
	var label string
	var q string
	if undo {
		q = `SELECT id, label FROM history_entries WHERE undone = 0 ORDER BY id DESC LIMIT 1`
	} else {
		q = `SELECT id, label FROM history_entries WHERE undone = 1 ORDER BY id ASC LIMIT 1`
	}
	if err := tx.QueryRowContext(ctx, q).Scan(&id, &label); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if undo {
				return "", ErrNothingToUndo
			}
			return "", ErrNothingToRedo
		}
		return "", err
	}
	changes, err := s.loadChanges(ctx, tx, id)
	if err != nil {
		return "", err
	}
	if undo {
		for i := len(changes) - 1; i >= 0; i-- {
			if err := applyInverse(ctx, tx, changes[i]); err != nil {
				return "", fmt.Errorf("annulation de %q: %w", label, err)
			}
		}
	} else {
		for _, c := range changes {
			if err := applyForward(ctx, tx, c); err != nil {
				return "", fmt.Errorf("rétablissement de %q: %w", label, err)
			}
		}
	}
	flag := 0
	if undo {
		flag = 1
	}
	if _, err := tx.ExecContext(ctx, `UPDATE history_entries SET undone = ? WHERE id = ?`, flag, id); err != nil {
		return "", err
	}
	return label, tx.Commit()
}

func applyInverse(ctx context.Context, x execer, c change) error {
	switch c.op {
	case "insert":
		return deleteRow(ctx, x, c.table, c.id)
	case "update":
		return updateRow(ctx, x, c.table, c.id, c.before)
	case "delete":
		return insertRow(ctx, x, c.table, c.before)
	}
	return fmt.Errorf("opération inconnue %q", c.op)
}

func applyForward(ctx context.Context, x execer, c change) error {
	switch c.op {
	case "insert":
		return insertRow(ctx, x, c.table, c.after)
	case "update":
		return updateRow(ctx, x, c.table, c.id, c.after)
	case "delete":
		return deleteRow(ctx, x, c.table, c.id)
	}
	return fmt.Errorf("opération inconnue %q", c.op)
}
