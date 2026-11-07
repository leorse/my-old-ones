package gedcom

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"moo/internal/domain"
)

// Options règle un import.
type Options struct {
	FileName string
	Progress func(done, total int, message string)
	Now      func() time.Time
}

// ImportFile lit, décode et importe un fichier GEDCOM dans la base ouverte.
// L'import est transactionnel : en cas d'erreur, rien n'est écrit. Il ajoute
// des données sans jamais modifier ni supprimer l'existant, et n'est pas
// journalisé dans l'historique annuler/rétablir (le projet est sauvegardé
// avant l'import par l'appelant).
func ImportFile(ctx context.Context, db *sql.DB, path string, opt Options) (*Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if opt.FileName == "" {
		opt.FileName = filepath.Base(path)
	}
	return ImportBytes(ctx, db, raw, opt)
}

// ImportBytes importe le contenu brut d'un fichier GEDCOM.
func ImportBytes(ctx context.Context, db *sql.DB, raw []byte, opt Options) (*Report, error) {
	start := time.Now()
	dec, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	records, perr := Parse(dec.Text)
	if len(records) == 0 || records[0].Tag != "HEAD" {
		return nil, fmt.Errorf("ce fichier n'est pas un GEDCOM valide (en-tête HEAD absent)")
	}
	rep := &Report{FileName: opt.FileName, Encoding: dec.Encoding, DeclaredEnc: dec.Declared}
	for _, w := range dec.Warnings {
		rep.warn("%s", w)
	}
	for _, w := range perr {
		rep.warn("%s", w)
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	im := newImporter(ctx, tx, rep, opt)
	if err := im.run(records); err != nil {
		return nil, err
	}
	rep.DurationMs = time.Since(start).Milliseconds()
	b, _ := json.Marshal(rep)
	if _, err := tx.ExecContext(ctx, `INSERT INTO gedcom_imports(imported_at, file_name, software, encoding, report) VALUES (?,?,?,?,?)`,
		opt.Now().UTC().Format(time.RFC3339), rep.FileName, rep.Software, rep.Encoding, string(b)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rep, nil
}

type importer struct {
	ctx  context.Context
	tx   *sql.Tx
	rep  *Report
	opt  Options
	now  string
	form []placeLevel

	stmts map[string]*sql.Stmt

	indi, fam, sour, note, repoRec map[string]*Node
	personID                       map[string]string
	personSex                      map[string]string
	sourceID                       map[string]string
	inlineSource                   map[string]string
	pedi                           map[string]string // "enfant|famille" -> PEDI

	placeIDs     map[string]string
	placeDisplay map[string]string
	placeHasGeo  map[string]bool

	parentsOf map[string][]string
	linkSeen  map[string]bool
	rolesUsed map[string]map[string]bool
	imported  []string // identifiants des personnes importées
}

func newImporter(ctx context.Context, tx *sql.Tx, rep *Report, opt Options) *importer {
	return &importer{
		ctx: ctx, tx: tx, rep: rep, opt: opt, now: opt.Now().UTC().Format(time.RFC3339),
		stmts: map[string]*sql.Stmt{},
		indi:  map[string]*Node{}, fam: map[string]*Node{}, sour: map[string]*Node{}, note: map[string]*Node{}, repoRec: map[string]*Node{},
		personID: map[string]string{}, personSex: map[string]string{}, sourceID: map[string]string{}, inlineSource: map[string]string{},
		pedi:     map[string]string{},
		placeIDs: map[string]string{}, placeDisplay: map[string]string{}, placeHasGeo: map[string]bool{},
		parentsOf: map[string][]string{}, linkSeen: map[string]bool{}, rolesUsed: map[string]map[string]bool{},
	}
}

func newID() string { return uuid.NewString() }

func (im *importer) insert(table string, row map[string]any) error {
	cols := make([]string, 0, len(row))
	for c := range row {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	key := table + ":" + strings.Join(cols, ",")
	st := im.stmts[key]
	if st == nil {
		q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ","),
			strings.TrimSuffix(strings.Repeat("?,", len(cols)), ","))
		var err error
		if st, err = im.tx.PrepareContext(im.ctx, q); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
		im.stmts[key] = st
	}
	args := make([]any, len(cols))
	for i, c := range cols {
		args[i] = row[c]
	}
	if _, err := st.ExecContext(im.ctx, args...); err != nil {
		return fmt.Errorf("insertion dans %s: %w", table, err)
	}
	return nil
}

func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nzi(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func (im *importer) run(records []*Node) error {
	head := records[0]
	im.rep.Software = strings.TrimSpace(strings.Join(nonEmpty(head.Val("SOUR"), head.Child("SOUR").val("NAME"), head.Child("SOUR").val("VERS")), " "))
	im.rep.GedcomVers = head.Child("GEDC").val("VERS")
	if p := head.Child("PLAC"); p != nil {
		im.form = formLevels(p.Val("FORM"))
	}

	var indis, fams []*Node
	for _, r := range records[1:] {
		switch r.Tag {
		case "INDI":
			indis = append(indis, r)
			im.indi[r.Xref] = r
		case "FAM":
			fams = append(fams, r)
			im.fam[r.Xref] = r
		case "SOUR":
			im.sour[r.Xref] = r
		case "NOTE":
			im.note[r.Xref] = r
		case "REPO":
			im.repoRec[r.Xref] = r
		case "OBJE":
			im.rep.MediaSkipped++
		case "TRLR", "SUBM", "SUBN", "SNOTE":
		default:
			im.rep.skip(r.Tag)
		}
	}
	// PEDI : lien d'un enfant à une famille
	for _, n := range indis {
		for _, f := range n.All("FAMC") {
			if p := Pointer(f.Value); p != "" {
				im.pedi[n.Xref+"|"+p] = strings.ToLower(f.Val("PEDI"))
			}
		}
	}

	total := len(indis) + len(fams)
	done := 0
	tick := func(msg string) {
		done++
		if im.opt.Progress != nil && (done%25 == 0 || done == total) {
			im.opt.Progress(done, total, msg)
		}
	}

	if err := im.sources(); err != nil {
		return err
	}
	for _, n := range indis {
		if err := im.ctx.Err(); err != nil {
			return err
		}
		if err := im.individual(n); err != nil {
			return fmt.Errorf("individu @%s@ (ligne %d): %w", n.Xref, n.Line, err)
		}
		tick("Personnes")
	}
	for _, n := range fams {
		if err := im.ctx.Err(); err != nil {
			return err
		}
		if err := im.family(n); err != nil {
			return fmt.Errorf("famille @%s@ (ligne %d): %w", n.Xref, n.Line, err)
		}
		tick("Familles")
	}
	return im.finalize()
}

// val : Node nil-safe.
func (n *Node) val(tag string) string {
	if n == nil {
		return ""
	}
	return n.Val(tag)
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// --- notes, extras ----------------------------------------------------------

func (im *importer) noteText(n *Node) string {
	if p := Pointer(n.Value); p != "" {
		if r := im.note[p]; r != nil {
			im.rep.Notes++
			return strings.TrimSpace(r.Value)
		}
		im.rep.warn("note @%s@ introuvable (ligne %d)", p, n.Line)
		return ""
	}
	im.rep.Notes++
	return strings.TrimSpace(n.Value)
}

func (im *importer) extra(table, id string, n *Node) error {
	im.rep.ExtraTags++
	return im.insert("gedcom_extras", map[string]any{"id": newID(), "owner_table": table, "owner_id": id, "raw": n.Raw(1)})
}

// --- lieux ------------------------------------------------------------------

var reHTTP = regexp.MustCompile(`https?://[^\s)\]]+`)

// placeFor crée (ou retrouve) la hiérarchie d'un lieu et renvoie l'identifiant du lieu le plus précis.
func (im *importer) placeFor(plac *Node) (string, error) {
	if plac == nil || strings.TrimSpace(plac.Value) == "" {
		return "", nil
	}
	steps := ParsePlace(plac.Value, im.form)
	if len(steps) == 0 {
		return "", nil
	}
	parent, key := "", ""
	var leaf string
	for _, st := range steps {
		key += "/" + st.Type + ":" + fold(st.Name)
		id, ok := im.placeIDs[key]
		if !ok {
			id = newID()
			display := st.Name
			if parent != "" {
				display = st.Name + ", " + im.placeDisplay[parent]
			}
			code := ""
			if st.Code != "" && reInsee.MatchString(st.Code) {
				code = st.Code
			}
			if err := im.insert("places", map[string]any{"id": id, "name": st.Name, "type": st.Type,
				"parent_id": nz(parent), "code_insee": nz(code), "display_name": display}); err != nil {
				return "", err
			}
			im.placeIDs[key], im.placeDisplay[id] = id, display
			im.rep.Places++
		}
		parent, leaf = id, id
	}
	if m := plac.Child("MAP"); m != nil && !im.placeHasGeo[leaf] {
		lat, ok1 := ParseCoord(m.Val("LATI"))
		lon, ok2 := ParseCoord(m.Val("LONG"))
		if ok1 && ok2 {
			if _, err := im.tx.ExecContext(im.ctx, `UPDATE places SET latitude = ?, longitude = ? WHERE id = ?`, lat, lon, leaf); err != nil {
				return "", err
			}
			im.placeHasGeo[leaf] = true
		}
	}
	return leaf, nil
}

// --- sources et citations ---------------------------------------------------

func (im *importer) sources() error {
	xrefs := make([]string, 0, len(im.sour))
	for x := range im.sour {
		xrefs = append(xrefs, x)
	}
	sort.Strings(xrefs)
	for _, x := range xrefs {
		n := im.sour[x]
		id := newID()
		im.sourceID[x] = id
		title := n.Val("TITL")
		if title == "" {
			title = n.Val("ABBR")
		}
		repoName := ""
		if r := n.Child("REPO"); r != nil {
			if p := Pointer(r.Value); p != "" && im.repoRec[p] != nil {
				repoName = im.repoRec[p].Val("NAME")
			} else {
				repoName = r.Value
			}
			if c := r.Val("CALN"); c != "" {
				repoName = strings.TrimSpace(repoName + " " + c)
			}
		}
		var notes []string
		url := ""
		for _, t := range n.Children {
			switch t.Tag {
			case "TEXT", "NOTE":
				txt := t.Value
				if t.Tag == "NOTE" {
					txt = im.noteText(t)
				}
				if txt != "" {
					notes = append(notes, txt)
				}
				if url == "" {
					url = reHTTP.FindString(txt)
				}
			case "TITL", "ABBR", "AUTH", "PUBL", "REPO", "TYPE", "DATA":
			case "WWW", "_URL", "URL":
				url = t.Value
			default:
				if err := im.extra("sources", id, t); err != nil {
					return err
				}
			}
		}
		typ := strings.ToLower(n.Val("TYPE"))
		if typ == "" {
			typ = "other"
		}
		if err := im.insert("sources", map[string]any{"id": id, "type": typ, "title": title, "author": n.Val("AUTH"),
			"publisher": n.Val("PUBL"), "date": n.Child("DATA").val("DATE"), "repository": repoName, "url": url,
			"notes_md": strings.Join(notes, "\n\n")}); err != nil {
			return err
		}
		im.rep.Sources++
	}
	return nil
}

// citation rattache une citation GEDCOM (SOUR) à un objet.
func (im *importer) citation(targetType, targetID string, s *Node) error {
	var sid string
	if p := Pointer(s.Value); p != "" {
		sid = im.sourceID[p]
		if sid == "" {
			im.rep.warn("source @%s@ introuvable (ligne %d)", p, s.Line)
			return nil
		}
	} else {
		title := strings.TrimSpace(s.Value)
		if title == "" {
			title = "(source sans titre)"
		}
		if sid = im.inlineSource[title]; sid == "" {
			sid = newID()
			if err := im.insert("sources", map[string]any{"id": sid, "type": "other", "title": title}); err != nil {
				return err
			}
			im.inlineSource[title] = sid
			im.rep.Sources++
		}
	}
	rel := 2
	if q := strings.TrimSpace(s.Val("QUAY")); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 0 && n <= 3 {
			rel = n
		}
	}
	quote := s.Child("DATA").val("TEXT")
	if quote == "" {
		quote = s.Val("TEXT")
	}
	im.rep.Citations++
	return im.insert("citations", map[string]any{"id": newID(), "source_id": sid, "target_type": targetType,
		"target_id": targetID, "detail": s.Val("PAGE"), "reliability": rel, "quote": quote, "origin": "gedcom"})
}

// --- événements -------------------------------------------------------------

var personEvents = map[string]string{
	"BIRT": "birth", "CHR": "baptism", "BAPM": "baptism", "DEAT": "death", "BURI": "burial", "CREM": "cremation",
	"OCCU": "occupation", "RESI": "residence", "CENS": "census", "EMIG": "emigration", "IMMI": "immigration",
	"NATU": "naturalization", "GRAD": "degree", "EDUC": "degree", "RELI": "religion",
}

// Événements/attributs sans type dédié : « personnalisé » avec un libellé.
var customLabels = map[string]string{
	"ADOP": "Adoption", "BARM": "Bar-mitsva", "BASM": "Bat-mitsva", "BLES": "Bénédiction", "CHRA": "Baptême adulte",
	"CONF": "Confirmation", "FCOM": "Première communion", "ORDN": "Ordination", "RETI": "Retraite",
	"WILL": "Testament", "PROB": "Homologation", "TITL": "Titre", "NATI": "Nationalité", "PROP": "Propriété",
	"SSN": "Numéro de sécurité sociale", "CAST": "Caste", "DSCR": "Description physique", "IDNO": "Numéro d'identité",
	"NCHI": "Nombre d'enfants", "NMR": "Nombre de mariages", "EVEN": "Événement", "FACT": "Fait",
	"MILI": "Service militaire",
}

// Tags dont la valeur est la description (attributs).
var attrTags = map[string]bool{"OCCU": true, "RESI": true, "EDUC": true, "GRAD": true, "RELI": true, "TITL": true,
	"NATI": true, "PROP": true, "SSN": true, "CAST": true, "DSCR": true, "IDNO": true, "FACT": true, "EVEN": true,
	"NCHI": true, "NMR": true, "MILI": true}

var unionEventTags = map[string]string{"MARR": "marriage", "DIV": "divorce"}
var unionCustomLabels = map[string]string{"ANUL": "Annulation", "ENGA": "Fiançailles", "MARB": "Publication des bans",
	"MARC": "Contrat de mariage", "MARL": "Licence de mariage", "MARS": "Régime matrimonial", "DIVF": "Demande de divorce",
	"EVEN": "Événement", "SEPA": "Séparation"}

func addressText(a *Node) string {
	if a == nil {
		return ""
	}
	parts := []string{strings.ReplaceAll(strings.TrimSpace(a.Value), "\n", ", ")}
	for _, t := range a.Children {
		if v := strings.TrimSpace(t.Value); v != "" && t.Tag != "PHON" && t.Tag != "EMAIL" {
			parts = append(parts, v)
		}
	}
	return strings.Join(nonEmpty(parts...), ", ")
}

// event enregistre un événement de personne (owner "persons") ou d'union (owner "unions").
func (im *importer) event(ownerTable, ownerID string, n *Node) (bool, error) {
	var typ, label string
	if ownerTable == "persons" {
		if t, ok := personEvents[n.Tag]; ok {
			typ = t
		} else if l, ok := customLabels[n.Tag]; ok {
			typ, label = "custom", l
		} else {
			return false, nil
		}
	} else {
		if t, ok := unionEventTags[n.Tag]; ok {
			typ = t
		} else if l, ok := unionCustomLabels[n.Tag]; ok {
			typ, label = "custom", l
		} else {
			return false, nil
		}
	}

	desc := ""
	value := strings.TrimSpace(n.Value)
	if strings.EqualFold(value, "Y") {
		value = ""
	}
	if attrTags[n.Tag] || n.Tag == "EVEN" {
		desc = value
	}
	if t := n.Val("TYPE"); t != "" && (typ == "custom" || n.Tag == "EVEN" || n.Tag == "FACT") {
		desc = strings.TrimSpace(t + " " + desc)
		if desc != t && !strings.HasSuffix(t, ":") {
			desc = t + " : " + strings.TrimSpace(strings.TrimPrefix(desc, t))
		}
	}
	if typ == "custom" && label != "" && label != "Événement" && label != "Fait" {
		if desc == "" {
			desc = label
		} else {
			desc = label + " : " + desc
		}
	}
	if desc == "" {
		desc = addressText(n.Child("ADDR"))
	}

	var notes []string
	for _, c := range n.All("NOTE") {
		if t := im.noteText(c); t != "" {
			notes = append(notes, t)
		}
	}
	if c := n.Val("CAUS"); c != "" {
		notes = append(notes, "Cause : "+c)
	}
	if a := n.Val("AGE"); a != "" {
		notes = append(notes, "Âge : "+a)
	}

	date := ParseDate(n.Val("DATE"))
	if date.Precision == domain.PrecText {
		im.rep.correct("date non interprétée conservée comme texte : « %s » (ligne %d)", date.OriginalText, n.Child("DATE").Line)
	}
	placeID, err := im.placeFor(n.Child("PLAC"))
	if err != nil {
		return true, err
	}
	sours := n.All("SOUR")
	empty := date.Precision == domain.PrecNone && placeID == "" && desc == "" && len(notes) == 0 && len(sours) == 0
	if empty && n.Tag != "DEAT" && len(n.Children) == 0 {
		im.rep.skip(n.Tag + " (vide)")
		return true, nil
	}

	id := newID()
	var key any
	if k, ok := date.SortKey(); ok {
		key = k
	}
	row := map[string]any{
		"id": id, "type": typ, "place_id": nz(placeID), "description": desc, "notes_md": strings.Join(notes, "\n\n"),
		"date_precision": string(date.Precision), "date_calendar": string(date.Calendar),
		"y1": nzi(date.Y1), "m1": nzi(date.M1), "d1": nzi(date.D1), "y2": nzi(date.Y2), "m2": nzi(date.M2), "d2": nzi(date.D2),
		"sort_key": key, "original_text": strings.TrimSpace(n.Val("DATE")),
	}
	if ownerTable == "persons" {
		row["person_id"] = ownerID
	} else {
		row["union_id"] = ownerID
	}
	if err := im.insert("events", row); err != nil {
		return true, err
	}
	im.rep.Events++

	for _, s := range sours {
		if err := im.citation("event", id, s); err != nil {
			return true, err
		}
	}
	for _, c := range n.Children {
		switch c.Tag {
		case "DATE", "PLAC", "ADDR", "NOTE", "SOUR", "TYPE", "CAUS", "AGE", "PHON", "EMAIL", "WWW":
		case "OBJE":
			im.rep.MediaSkipped++
			if err := im.extra("events", id, c); err != nil {
				return true, err
			}
		default:
			if err := im.extra("events", id, c); err != nil {
				return true, err
			}
		}
	}
	return true, nil
}

// --- individus --------------------------------------------------------------

func parseNameValue(v string) (given, surname, suffix string) {
	i := strings.Index(v, "/")
	if i < 0 {
		return strings.TrimSpace(v), "", ""
	}
	j := strings.Index(v[i+1:], "/")
	if j < 0 {
		return strings.TrimSpace(v[:i]), strings.TrimSpace(v[i+1:]), ""
	}
	j += i + 1
	return strings.TrimSpace(v[:i]), strings.TrimSpace(v[i+1 : j]), strings.TrimSpace(v[j+1:])
}

var nameTypes = map[string]string{"birth": "birth", "married": "marital", "marital": "marital", "aka": "usage",
	"religious": "religious", "nickname": "nickname"}

func (im *importer) insertName(pid string, n *Node, typ string, primary bool, given, surname, prefix, suffix string) (string, error) {
	id := newID()
	g := strings.Fields(given)
	if g == nil {
		g = []string{}
	}
	gj, _ := json.Marshal(g)
	prim := 0
	if primary {
		prim = 1
	}
	im.rep.Names++
	return id, im.insert("names", map[string]any{"id": id, "person_id": pid, "type": typ, "surname": surname,
		"given_names": string(gj), "usual_given": 0, "prefix": prefix, "suffix": suffix, "is_primary": prim})
}

func (im *importer) individual(n *Node) error {
	pid := newID()
	im.personID[n.Xref] = pid
	im.imported = append(im.imported, pid)

	sex := "unknown"
	switch strings.ToUpper(strings.TrimSpace(n.Val("SEX"))) {
	case "M":
		sex = "M"
	case "F":
		sex = "F"
	case "X", "O":
		sex = "other"
	}
	im.personSex[pid] = sex

	updated := im.now
	if c := n.Child("CHAN"); c != nil {
		if t, ok := parseChan(c.Val("DATE"), c.Child("DATE").val("TIME")); ok {
			updated = t
		}
	}
	var notes []string
	for _, c := range n.All("NOTE") {
		if t := im.noteText(c); t != "" {
			notes = append(notes, t)
		}
	}
	if err := im.insert("persons", map[string]any{"id": pid, "sex": sex, "notes_md": strings.Join(notes, "\n\n"),
		"created_at": im.now, "updated_at": updated}); err != nil {
		return err
	}
	im.rep.Individuals++

	named := false
	for _, c := range n.Children {
		switch c.Tag {
		case "NAME":
			if err := im.name(pid, c, !named); err != nil {
				return err
			}
			named = true
		case "SEX", "FAMC", "FAMS", "CHAN", "NOTE":
		case "SOUR":
			if err := im.citation("person", pid, c); err != nil {
				return err
			}
		case "OBJE":
			im.rep.MediaSkipped++
			if err := im.extra("persons", pid, c); err != nil {
				return err
			}
		default:
			handled, err := im.event("persons", pid, c)
			if err != nil {
				return err
			}
			if !handled {
				if err := im.extra("persons", pid, c); err != nil {
					return err
				}
			}
		}
	}
	if !named { // la liste des personnes suppose un nom principal
		if _, err := im.insertName(pid, nil, "birth", true, "", "", "", ""); err != nil {
			return err
		}
		im.rep.correct("personne @%s@ sans nom : nom vide créé", n.Xref)
	}
	return nil
}

func (im *importer) name(pid string, n *Node, primary bool) error {
	given, surname, suffix := parseNameValue(n.Value)
	if g := n.Val("GIVN"); g != "" {
		given = g
	}
	if s := n.Val("SURN"); s != "" {
		surname = s
	}
	if s := n.Val("NSFX"); s != "" {
		suffix = s
	}
	if sp := n.Val("SPFX"); sp != "" && !strings.HasPrefix(strings.ToLower(surname), strings.ToLower(sp)) {
		surname = strings.TrimSpace(sp + " " + surname)
	}
	typ := nameTypes[strings.ToLower(strings.TrimSpace(n.Val("TYPE")))]
	if typ == "" {
		typ = "other"
		if primary {
			typ = "birth"
		}
	}
	id, err := im.insertName(pid, n, typ, primary, given, surname, n.Val("NPFX"), suffix)
	if err != nil {
		return err
	}
	for _, c := range n.Children {
		switch c.Tag {
		case "GIVN", "SURN", "NSFX", "SPFX", "NPFX", "TYPE", "NOTE":
		case "SOUR":
			if err := im.citation("name", id, c); err != nil {
				return err
			}
		case "NICK":
			if _, err := im.insertName(pid, nil, "nickname", false, c.Value, "", "", ""); err != nil {
				return err
			}
		case "_MARNM":
			g2, s2, x2 := parseNameValue(c.Value)
			if _, err := im.insertName(pid, nil, "marital", false, g2, s2, "", x2); err != nil {
				return err
			}
		default:
			if err := im.extra("names", id, c); err != nil {
				return err
			}
		}
	}
	return nil
}

var reDay = regexp.MustCompile(`^\d{1,2}$`)

func parseChan(date, tm string) (string, bool) {
	d := ParseDate(date)
	if d.Precision != domain.PrecExact || d.Calendar != domain.CalGregorian {
		return "", false
	}
	h, m := 0, 0
	if f := strings.Split(strings.TrimSpace(tm), ":"); len(f) >= 2 && reDay.MatchString(f[0]) {
		h, _ = strconv.Atoi(f[0])
		m, _ = strconv.Atoi(f[1])
	}
	t := time.Date(d.Y1, time.Month(d.M1), d.D1, h, m, 0, 0, time.UTC)
	return t.Format(time.RFC3339), true
}

// --- familles ---------------------------------------------------------------

// wouldCycle : child est-il un ancêtre de parent (lier créerait une boucle) ?
func (im *importer) wouldCycle(child, parent string) bool {
	seen := map[string]bool{}
	stack := []string{parent}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x == child {
			return true
		}
		if seen[x] {
			continue
		}
		seen[x] = true
		stack = append(stack, im.parentsOf[x]...)
	}
	return false
}

func (im *importer) family(n *Node) error {
	var partners []string
	for _, tag := range []string{"HUSB", "WIFE"} {
		for _, h := range n.All(tag) {
			p := Pointer(h.Value)
			pid := im.personID[p]
			if pid == "" {
				im.rep.warn("famille @%s@ : %s @%s@ introuvable (ligne %d)", n.Xref, tag, p, h.Line)
				continue
			}
			dup := false
			for _, x := range partners {
				dup = dup || x == pid
			}
			if dup {
				im.rep.warn("famille @%s@ : la même personne est citée deux fois comme conjoint", n.Xref)
				continue
			}
			partners = append(partners, pid)
		}
	}

	hasMarr, ended, hasUnionEvent := false, false, false
	typ := "other"
	for _, c := range n.Children {
		switch c.Tag {
		case "MARR":
			hasMarr, hasUnionEvent = true, true
			if strings.Contains(strings.ToLower(c.Val("TYPE")), "pacs") {
				typ = "pacs"
			} else if typ != "pacs" {
				typ = "marriage"
			}
		case "DIV", "ANUL", "DIVF":
			ended, hasUnionEvent = true, true
		}
	}
	unionID := ""
	if len(partners) >= 2 || hasUnionEvent {
		if !hasMarr && len(partners) >= 2 {
			typ = "free"
		}
		status := "ongoing"
		if ended {
			status = "ended"
		}
		unionID = newID()
		var notes []string
		for _, c := range n.All("NOTE") {
			if t := im.noteText(c); t != "" {
				notes = append(notes, t)
			}
		}
		if err := im.insert("unions", map[string]any{"id": unionID, "type": typ, "status": status,
			"notes_md": strings.Join(notes, "\n\n"), "created_at": im.now, "updated_at": im.now}); err != nil {
			return err
		}
		im.rep.Unions++
		for i, p := range partners {
			if err := im.insert("union_partners", map[string]any{"id": newID(), "union_id": unionID, "person_id": p, "position": i}); err != nil {
				return err
			}
		}
	}
	im.rep.Families++

	for _, c := range n.Children {
		switch c.Tag {
		case "HUSB", "WIFE", "CHIL", "NOTE", "NCHI":
		case "SOUR":
			if unionID != "" {
				if err := im.citation("union", unionID, c); err != nil {
					return err
				}
			}
		default:
			if unionID == "" {
				im.rep.skip(c.Tag + " (famille sans union)")
				continue
			}
			handled, err := im.event("unions", unionID, c)
			if err != nil {
				return err
			}
			if !handled {
				if err := im.extra("unions", unionID, c); err != nil {
					return err
				}
			}
		}
	}

	order := 0
	for _, ch := range n.All("CHIL") {
		cx := Pointer(ch.Value)
		child := im.personID[cx]
		if child == "" {
			im.rep.warn("famille @%s@ : enfant @%s@ introuvable (ligne %d)", n.Xref, cx, ch.Line)
			continue
		}
		order++
		kind := "biological"
		switch im.pedi[cx+"|"+n.Xref] {
		case "adopted":
			kind = "adoptive"
		case "foster":
			kind = "foster"
		case "sealing":
			kind = "legal"
		case "step":
			kind = "step"
		}
		for _, parent := range partners {
			key := child + "|" + parent
			switch {
			case parent == child:
				im.rep.warn("famille @%s@ : @%s@ est son propre parent, lien ignoré", n.Xref, cx)
				continue
			case im.linkSeen[key]:
				im.rep.warn("famille @%s@ : lien de filiation en double pour @%s@, ignoré", n.Xref, cx)
				continue
			case im.wouldCycle(child, parent):
				im.rep.warn("famille @%s@ : lien ignoré, @%s@ serait son propre ancêtre (cycle)", n.Xref, cx)
				continue
			}
			role := "parent"
			switch im.personSex[parent] {
			case "M":
				role = "father"
			case "F":
				role = "mother"
			}
			used := im.rolesUsed[child]
			if used == nil {
				used = map[string]bool{}
				im.rolesUsed[child] = used
			}
			if kind == "biological" && role != "parent" {
				if used[role] {
					role = "parent"
				} else {
					used[role] = true
				}
			}
			if err := im.insert("parent_child", map[string]any{"id": newID(), "child_id": child, "parent_id": parent,
				"parent_role": role, "kind": kind, "union_id": nz(unionID), "birth_order": order}); err != nil {
				return err
			}
			im.linkSeen[key] = true
			im.parentsOf[child] = append(im.parentsOf[child], parent)
			im.rep.ParentLinks++
		}
	}
	return nil
}

// --- finalisation -----------------------------------------------------------

func (im *importer) finalize() error {
	ids, _ := json.Marshal(im.imported)
	// un décès, une inhumation ou une crémation implique « décédé »
	if _, err := im.tx.ExecContext(im.ctx, `UPDATE persons SET living = 'deceased'
		WHERE living = 'unknown' AND id IN (SELECT value FROM json_each(?1))
		AND id IN (SELECT person_id FROM events WHERE type IN ('death','burial','cremation'))`, string(ids)); err != nil {
		return err
	}
	// né il y a plus de 110 ans : considéré décédé
	limit := (im.opt.Now().Year() - 110) * 10000
	res, err := im.tx.ExecContext(im.ctx, `UPDATE persons SET living = 'deceased'
		WHERE living = 'unknown' AND id IN (SELECT value FROM json_each(?1))
		AND id IN (SELECT person_id FROM events WHERE type = 'birth' AND sort_key IS NOT NULL AND sort_key <= ?2 + 1231)`,
		string(ids), limit)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		im.rep.AutoDeceased = int(n)
		im.rep.correct("%d personne(s) nées il y a plus de 110 ans marquées décédées", n)
	}
	for _, st := range im.stmts {
		st.Close()
	}
	im.stmts = map[string]*sql.Stmt{}
	return nil
}
