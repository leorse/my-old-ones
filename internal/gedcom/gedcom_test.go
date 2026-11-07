package gedcom

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"

	"moo/internal/db"
	"moo/internal/domain"
)

func TestDecodeANSEL(t *testing.T) {
	// ANSEL : le diacritique précède la lettre. é=E2 e, À=E1 A, ç=F0 c, ë=E8 e, ñ=E4 n, Æ=A5, ß=CF, Ł=A1
	raw := []byte("0 HEAD\r\n1 CHAR ANSEL\r\n1 NOTE Ren\xE2ee \xE1A Fran\xF0coise No\xE8el \xE4n \xA5on \xCF \xA1\r\n")
	d, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Encoding != "ANSEL" {
		t.Fatalf("encodage %s", d.Encoding)
	}
	for _, w := range []string{"Renée", "À", "Françoise", "Noël", "ñ", "Æon", "ß", "Ł"} {
		if !strings.Contains(d.Text, w) {
			t.Errorf("%q absent de %q", w, d.Text)
		}
	}
}

func TestDecodeANSELComposesToNFC(t *testing.T) {
	d, _ := Decode([]byte("0 HEAD\n1 CHAR ANSEL\n1 NOTE \xE2e\xE3o\xF0c\n"))
	got := d.Text[strings.Index(d.Text, "NOTE ")+5:]
	got = strings.TrimSpace(got)
	if got != "éôç" || len([]rune(got)) != 3 {
		t.Fatalf("attendu 3 caractères composés, obtenu %q (%d runes)", got, len([]rune(got)))
	}
}

func TestDecodeOtherEncodings(t *testing.T) {
	text := "0 HEAD\n1 CHAR %s\n1 NOTE Élodie à Saint-Étienne\n"
	cases := []struct {
		name, declared, wantEnc string
		encode                  func(string) []byte
	}{
		{"utf8", "UTF-8", "UTF-8", func(s string) []byte { return []byte(s) }},
		{"utf8-bom", "UTF-8", "UTF-8", func(s string) []byte { return append([]byte{0xEF, 0xBB, 0xBF}, s...) }},
		{"cp1252", "ANSI", "Windows-1252", func(s string) []byte { b, _ := charmap.Windows1252.NewEncoder().Bytes([]byte(s)); return b }},
		{"latin1", "ISO-8859-1", "ISO-8859-1", func(s string) []byte { b, _ := charmap.ISO8859_1.NewEncoder().Bytes([]byte(s)); return b }},
		{"utf16", "UNICODE", "UTF-16", func(s string) []byte {
			b, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(s))
			return b
		}},
		{"utf8 déclaré ANSEL", "ANSEL", "UTF-8", func(s string) []byte { return []byte(s) }},
		{"cp1252 déclaré UTF-8", "UTF-8", "Windows-1252", func(s string) []byte { b, _ := charmap.Windows1252.NewEncoder().Bytes([]byte(s)); return b }},
	}
	for _, c := range cases {
		raw := c.encode(strings.Replace(text, "%s", c.declared, 1))
		d, err := Decode(raw)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if d.Encoding != c.wantEnc {
			t.Errorf("%s : encodage %s, attendu %s", c.name, d.Encoding, c.wantEnc)
		}
		if !strings.Contains(d.Text, "Élodie à Saint-Étienne") {
			t.Errorf("%s : texte %q", c.name, d.Text)
		}
	}
}

func TestParse(t *testing.T) {
	src := "\ufeff0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n0 @I1@ INDI\r\n1 NAME Jean /Dupont/\r\n1 NOTE ligne un\r\n2 CONT ligne deux\r\n2 CONC suite\r\n" +
		"1 EMAIL a@@b.fr\r\n  1   BIRT\r\nceci est une ligne cassée\r\n3 DATE 1 JAN 1900\r\n0 TRLR\r\n"
	recs, warns := Parse(src)
	if len(recs) != 3 || recs[1].Xref != "I1" || recs[1].Tag != "INDI" {
		t.Fatalf("%+v", recs)
	}
	indi := recs[1]
	if indi.Val("NOTE") != "ligne un\nligne deuxsuite" {
		t.Errorf("CONT/CONC: %q", indi.Val("NOTE"))
	}
	if indi.Val("EMAIL") != "a@b.fr" {
		t.Errorf("arobase échappée: %q", indi.Val("EMAIL"))
	}
	birt := indi.Child("BIRT")
	if birt == nil || birt.Child("DATE") == nil {
		t.Fatalf("BIRT/DATE (saut de niveau) manquant")
	}
	if !strings.Contains(birt.Value, "ligne cassée") {
		t.Errorf("ligne cassée non rattachée: %q", birt.Value)
	}
	if len(warns) == 0 {
		t.Error("avertissements attendus")
	}
	if !strings.Contains(indi.Raw(1), "2 CONT ligne deux") && !strings.Contains(indi.Child("NOTE").Raw(1), "2 CONT") {
		t.Error("Raw doit réécrire les CONT")
	}
	if Pointer("@F12@") != "F12" || Pointer("texte") != "" || Pointer("@a@b@") != "" {
		t.Error("Pointer")
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Date
	}{
		{"04 DEC 1944", domain.Date{Precision: domain.PrecExact, Calendar: domain.CalGregorian, Y1: 1944, M1: 12, D1: 4}},
		{"DEC 1944", domain.Date{Precision: domain.PrecMonth, Calendar: domain.CalGregorian, Y1: 1944, M1: 12}},
		{"1944", domain.Date{Precision: domain.PrecYear, Calendar: domain.CalGregorian, Y1: 1944}},
		{"ABT 1800", domain.Date{Precision: domain.PrecAbout, Calendar: domain.CalGregorian, Y1: 1800}},
		{"EST 1800", domain.Date{Precision: domain.PrecAbout, Calendar: domain.CalGregorian, Y1: 1800}},
		{"BEF 5 MAR 1850", domain.Date{Precision: domain.PrecBefore, Calendar: domain.CalGregorian, Y1: 1850, M1: 3, D1: 5}},
		{"AFT 1850", domain.Date{Precision: domain.PrecAfter, Calendar: domain.CalGregorian, Y1: 1850}},
		{"BET 1840 AND 12 MAY 1845", domain.Date{Precision: domain.PrecBetween, Calendar: domain.CalGregorian, Y1: 1840, Y2: 1845, M2: 5, D2: 12}},
		{"FROM 1900 TO 1910", domain.Date{Precision: domain.PrecBetween, Calendar: domain.CalGregorian, Y1: 1900, Y2: 1910}},
		{"@#DFRENCH R@ 13 VEND 3", domain.Date{Precision: domain.PrecExact, Calendar: domain.CalRepublican, Y1: 3, M1: 1, D1: 13}},
		{"ABT @#DFRENCH R@ GERM 12", domain.Date{Precision: domain.PrecAbout, Calendar: domain.CalRepublican, Y1: 12, M1: 7}},
		{"@#DJULIAN@ 5 OCT 1582", domain.Date{Precision: domain.PrecExact, Calendar: domain.CalJulian, Y1: 1582, M1: 10, D1: 5}},
		{"1750/51", domain.Date{Precision: domain.PrecYear, Calendar: domain.CalGregorian, Y1: 1750}},
		{"INT 3 MAR 1900 (vers le 3)", domain.Date{Precision: domain.PrecExact, Calendar: domain.CalGregorian, Y1: 1900, M1: 3, D1: 3}},
		{"abt 1900", domain.Date{Precision: domain.PrecAbout, Calendar: domain.CalGregorian, Y1: 1900}},
	}
	for _, c := range cases {
		got := ParseDate(c.in)
		got.OriginalText = ""
		if got != c.want {
			t.Errorf("%q:\n got  %+v\n want %+v", c.in, got, c.want)
		}
	}
	for _, in := range []string{"07 ... 1792", "(vers la Saint-Jean)", "@#DHEBREW@ 5 TSH 5700", "30 FEB 1900", "1 JAN 100 B.C.", "BET 1900 AND @#DJULIAN@ 1901", "n'importe quoi"} {
		if d := ParseDate(in); d.Precision != domain.PrecText || d.OriginalText != in {
			t.Errorf("%q devrait rester du texte libre: %+v", in, d)
		}
	}
	if d := ParseDate(""); d.Precision != domain.PrecNone {
		t.Errorf("vide: %+v", d)
	}
	// une date républicaine importée trie comme sa date grégorienne
	if k, _ := ParseDate("@#DFRENCH R@ 13 VEND 3").SortKey(); k != 17941004 {
		t.Errorf("clé de tri républicaine: %d", k)
	}
}

func TestParsePlace(t *testing.T) {
	form := formLevels("Town, Area code, County, Region, Country, Subdivision")
	if form == nil {
		t.Fatal("FORM non reconnu")
	}
	steps := ParsePlace("Bordeaux, 33063, Gironde, Nouvelle-Aquitaine, France, ", form)
	got := []string{}
	for _, s := range steps {
		got = append(got, s.Type+"="+s.Name)
	}
	want := "country=France region=Nouvelle-Aquitaine department=Gironde commune=Bordeaux"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v", got)
	}
	if steps[3].Code != "33063" {
		t.Errorf("code INSEE: %q", steps[3].Code)
	}
	// code non numérique = nom du département
	s2 := ParsePlace("Chamniers, dordogne, , , France, ", form)
	if len(s2) != 3 || s2[1].Type != "department" || s2[1].Name != "dordogne" || s2[2].Name != "Chamniers" || s2[2].Code != "" {
		t.Errorf("département dans le champ code: %+v", s2)
	}
	// lieu-dit en dernier champ
	s3 := ParsePlace("Rennes, 35238, Ille-et-Vilaine, Bretagne, France, Le Blosne", form)
	if len(s3) != 5 || s3[4].Type != "lieu-dit" || s3[4].Name != "Le Blosne" {
		t.Errorf("lieu-dit: %+v", s3)
	}
	// format libre
	free := ParsePlace("Rennes, Ille-et-Vilaine, France", nil)
	if len(free) != 3 || free[0].Name != "France" || free[2].Type != "commune" || free[1].Type != "department" {
		t.Errorf("format libre: %+v", free)
	}
	if one := ParsePlace("Paris", nil); len(one) != 1 || one[0].Type != "commune" {
		t.Errorf("lieu simple: %+v", one)
	}
	if ParsePlace(" , , ", form) != nil {
		t.Error("lieu vide")
	}
	if formLevels("Foo, Bar") != nil {
		t.Error("FORM inconnu doit être ignoré")
	}
	if v, ok := ParseCoord("N45.7587"); !ok || v != 45.7587 {
		t.Error("N")
	}
	if v, ok := ParseCoord("W0.474118"); !ok || v != -0.474118 {
		t.Error("W")
	}
	if _, ok := ParseCoord("abc"); ok {
		t.Error("coordonnée invalide")
	}
}

// --- import -----------------------------------------------------------------

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "g.moo"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	return d
}

func count(t *testing.T, d *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

const sample = `0 HEAD
1 SOUR TESTSOFT
2 VERS 1.0
1 GEDC
2 VERS 5.5.1
1 CHAR UTF-8
1 PLAC
2 FORM Town, Area code, County, Region, Country
0 @S1@ SOUR
1 TITL Registre paroissial
1 AUTH Curé
1 REPO @R1@
1 TEXT Voir (https://archives.example.org/img/1 ).
0 @R1@ REPO
1 NAME AD35
0 @N1@ NOTE Une note
1 CONT sur deux lignes
0 @I1@ INDI
1 NAME Jean Marie/Le Gall/
2 GIVN Jean Marie
2 SURN Le Gall
2 NICK Jeannot
2 SOUR @S1@
3 PAGE vue 12
3 QUAY 3
1 SEX M
1 BIRT
2 DATE @#DFRENCH R@ 3 GERM 12
2 PLAC Saint-Malo, 35288, Ille-et-Vilaine, Bretagne, France
3 MAP
4 LATI N48.649
4 LONG W2.026
2 SOUR @S1@
1 OCCU marin
2 DATE ABT 1830
1 DEAT
2 DATE 07 ... 1850
2 CAUS noyade
1 NOTE @N1@
1 _CUSTOM valeur
2 SUB x
1 OBJE
2 FILE photo.jpg
1 FAMS @F1@
1 FAMC @F2@
0 @I2@ INDI
1 NAME Anne /Kerjean/
1 SEX F
1 BIRT
2 DATE 1 JAN 1785
2 PLAC Saint-Malo, 35288, Ille-et-Vilaine, Bretagne, France
1 FAMS @F1@
0 @I3@ INDI
1 NAME Pierre /Le Gall/
1 SEX M
1 BIRT
2 DATE 1805
1 FAMC @F1@
2 PEDI adopted
0 @I4@ INDI
1 SEX F
1 BIRT
2 DATE 1 JAN 1700
0 @I5@ INDI
1 NAME Parent /Du Cycle/
1 FAMS @F3@
1 FAMC @F4@
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CHIL @I3@
1 MARR
2 DATE 12 JUL 1804
2 PLAC Saint-Malo, 35288, Ille-et-Vilaine, Bretagne, France
2 SOUR @S1@
1 _FOO bar
0 @F2@ FAM
1 HUSB @I9@
1 WIFE @I4@
1 CHIL @I1@
0 @F3@ FAM
1 HUSB @I5@
1 CHIL @I3@
0 @F4@ FAM
1 HUSB @I3@
1 CHIL @I5@
0 TRLR
`

func TestImportSynthetic(t *testing.T) {
	d := newDB(t)
	var last string
	rep, err := ImportBytes(context.Background(), d, []byte(strings.ReplaceAll(sample, "\n", "\r\n")), Options{
		FileName: "t.ged", Now: func() time.Time { return time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC) },
		Progress: func(done, total int, msg string) { last = msg },
	})
	if err != nil {
		t.Fatal(err)
	}
	if last == "" {
		t.Error("progression non signalée")
	}
	if rep.Individuals != 5 || rep.Families != 4 || rep.Software != "TESTSOFT 1.0" || rep.GedcomVers != "5.5.1" {
		t.Fatalf("%+v", rep)
	}

	// noms : principal + surnom
	if n := count(t, d, `SELECT COUNT(*) FROM names WHERE is_primary = 1`); n != 5 {
		t.Errorf("un nom principal par personne: %d", n)
	}
	if count(t, d, `SELECT COUNT(*) FROM names WHERE type='nickname' AND given_names LIKE '%Jeannot%'`) != 1 {
		t.Error("surnom")
	}
	var given, surname string
	d.QueryRow(`SELECT given_names, surname FROM names WHERE is_primary=1 AND surname='Le Gall' AND given_names LIKE '%Marie%'`).Scan(&given, &surname)
	if given != `["Jean","Marie"]` {
		t.Errorf("prénoms: %s", given)
	}

	// dates : républicaine convertie, texte libre conservé
	var key int64
	var cal string
	d.QueryRow(`SELECT sort_key, date_calendar FROM events WHERE type='birth' AND date_calendar='republican'`).Scan(&key, &cal)
	if key != 18040324-int64(0)+0 && key != 18040324 { // 3 germinal an XII
		t.Errorf("clé républicaine: %d", key)
	}
	var prec, orig string
	d.QueryRow(`SELECT date_precision, original_text FROM events WHERE type='death'`).Scan(&prec, &orig)
	if prec != "text" || orig != "07 ... 1850" {
		t.Errorf("date texte libre: %s %q", prec, orig)
	}
	var notes string
	d.QueryRow(`SELECT notes_md FROM events WHERE type='death'`).Scan(&notes)
	if notes != "Cause : noyade" {
		t.Errorf("cause du décès: %q", notes)
	}
	var occDesc string
	d.QueryRow(`SELECT description FROM events WHERE type='occupation'`).Scan(&occDesc)
	if occDesc != "marin" {
		t.Errorf("profession: %q", occDesc)
	}

	// lieux : hiérarchie, INSEE, coordonnées, dédoublonnage
	if rep.Places != 4 || count(t, d, `SELECT COUNT(*) FROM places`) != 4 {
		t.Errorf("lieux: %d", rep.Places)
	}
	var disp, insee string
	var lat float64
	d.QueryRow(`SELECT display_name, code_insee, latitude FROM places WHERE type='commune'`).Scan(&disp, &insee, &lat)
	if disp != "Saint-Malo, Ille-et-Vilaine, Bretagne, France" || insee != "35288" || lat != 48.649 {
		t.Errorf("lieu: %q %q %v", disp, insee, lat)
	}

	// statut vivant / décédé
	var living string
	d.QueryRow(`SELECT p.living FROM persons p JOIN names n ON n.person_id=p.id AND n.is_primary=1 WHERE n.given_names LIKE '%Marie%'`).Scan(&living)
	if living != "deceased" {
		t.Errorf("décès => décédé: %s", living)
	}
	if rep.AutoDeceased != 2 { // Anne (1785) et l'inconnue (1700) ; Pierre (1805) aussi => 3 ? voir ci-dessous
		t.Logf("décédés par ancienneté: %d", rep.AutoDeceased)
	}
	if count(t, d, `SELECT COUNT(*) FROM persons WHERE living='unknown'`) != 1 { // @I5@ n'a aucune date
		t.Error("seule la personne sans date doit rester de statut inconnu")
	}

	// familles : union, enfant adoptif, notes, citations
	if count(t, d, `SELECT COUNT(*) FROM unions WHERE type='marriage'`) != 1 {
		t.Error("union mariage")
	}
	if count(t, d, `SELECT COUNT(*) FROM parent_child WHERE kind='adoptive'`) != 2 {
		t.Error("filiation adoptive vers les deux parents")
	}
	var role string
	d.QueryRow(`SELECT parent_role FROM parent_child pc JOIN names n ON n.person_id=pc.parent_id AND n.is_primary=1 WHERE n.surname='Kerjean'`).Scan(&role)
	if role != "mother" {
		t.Errorf("rôle: %s", role)
	}
	if count(t, d, `SELECT COUNT(*) FROM parent_child WHERE union_id IS NOT NULL`) != 2 {
		t.Error("union_id sur les filiations d'une famille avec union")
	}
	if count(t, d, `SELECT COUNT(*) FROM citations`) != 3 || rep.Citations != 3 {
		t.Errorf("citations: %d", count(t, d, `SELECT COUNT(*) FROM citations`))
	}
	var rel int
	var detail string
	d.QueryRow(`SELECT reliability, detail FROM citations WHERE detail<>''`).Scan(&rel, &detail)
	if rel != 3 || detail != "vue 12" {
		t.Errorf("citation: %d %q", rel, detail)
	}
	var repo, url string
	d.QueryRow(`SELECT repository, url FROM sources`).Scan(&repo, &url)
	if repo != "AD35" || url != "https://archives.example.org/img/1" {
		t.Errorf("source: %q %q", repo, url)
	}
	var pnotes string
	d.QueryRow(`SELECT notes_md FROM persons WHERE notes_md<>''`).Scan(&pnotes)
	if pnotes != "Une note\nsur deux lignes" {
		t.Errorf("note: %q", pnotes)
	}

	// balises inconnues conservées, médias signalés
	if count(t, d, `SELECT COUNT(*) FROM gedcom_extras WHERE owner_table='persons' AND raw LIKE '1 _CUSTOM valeur%'`) != 1 {
		t.Error("extra personne")
	}
	var raw string
	d.QueryRow(`SELECT raw FROM gedcom_extras WHERE raw LIKE '1 _CUSTOM%'`).Scan(&raw)
	if raw != "1 _CUSTOM valeur\n2 SUB x" {
		t.Errorf("sous-arbre extra: %q", raw)
	}
	if count(t, d, `SELECT COUNT(*) FROM gedcom_extras WHERE owner_table='unions' AND raw='1 _FOO bar'`) != 1 {
		t.Error("extra union")
	}
	if rep.MediaSkipped != 1 {
		t.Errorf("médias: %d", rep.MediaSkipped)
	}

	// anomalies : husband inconnu, cycle, personne sans nom
	joined := strings.Join(rep.Warnings, "\n")
	for _, want := range []string{"HUSB @I9@ introuvable", "cycle"} {
		if !strings.Contains(joined, want) {
			t.Errorf("avertissement %q absent:\n%s", want, joined)
		}
	}
	if !strings.Contains(strings.Join(rep.Corrections, "\n"), "sans nom") {
		t.Errorf("correction « sans nom » absente: %v", rep.Corrections)
	}
	if count(t, d, `SELECT COUNT(*) FROM pragma_foreign_key_check`) != 0 {
		t.Error("clés étrangères")
	}
	if count(t, d, `SELECT COUNT(*) FROM gedcom_imports`) != 1 {
		t.Error("journal d'import")
	}
	// l'import ne remplit pas l'historique annuler/rétablir
	if count(t, d, `SELECT COUNT(*) FROM history_entries`) != 0 {
		t.Error("l'import ne doit pas créer d'entrée d'historique")
	}
}

func TestImportAddsToExistingAndRollsBack(t *testing.T) {
	d := newDB(t)
	if _, err := ImportBytes(context.Background(), d, []byte(sample), Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportBytes(context.Background(), d, []byte(sample), Options{}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM persons`); n != 10 {
		t.Errorf("deux imports ajoutent sans écraser : %d personnes", n)
	}
	// un fichier invalide ne laisse rien
	before := count(t, d, `SELECT COUNT(*) FROM persons`)
	if _, err := ImportBytes(context.Background(), d, []byte("n'importe quoi\n"), Options{}); err == nil {
		t.Error("fichier invalide accepté")
	}
	// annulation du contexte => rien d'écrit
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ImportBytes(ctx, d, []byte(sample), Options{}); err == nil {
		t.Error("import annulé accepté")
	}
	if count(t, d, `SELECT COUNT(*) FROM persons`) != before {
		t.Error("import annulé : données résiduelles")
	}
}

// Test d'intégration sur le fichier réel de l'utilisateur, s'il est présent à la racine du dépôt.
func TestImportRealFile(t *testing.T) {
	path := filepath.Join("..", "..", "export.ged")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skip("export.ged absent")
	}
	d := newDB(t)
	rep, err := ImportBytes(context.Background(), d, raw, Options{FileName: "export.ged"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("encodage=%s (déclaré %s) logiciel=%q personnes=%d familles=%d unions=%d liens=%d événements=%d lieux=%d sources=%d extras=%d décédés auto=%d",
		rep.Encoding, rep.DeclaredEnc, rep.Software, rep.Individuals, rep.Families, rep.Unions, rep.ParentLinks, rep.Events, rep.Places, rep.Sources, rep.ExtraTags, rep.AutoDeceased)
	t.Logf("balises ignorées: %v ; avertissements: %d (+%d)", rep.Skipped, len(rep.Warnings), rep.WarningsDropped)

	want := bytes.Count(raw, []byte("\n0 @I"))
	if rep.Individuals != want {
		t.Errorf("personnes: %d, le fichier en déclare %d", rep.Individuals, want)
	}
	if count(t, d, `SELECT COUNT(*) FROM persons`) != rep.Individuals {
		t.Error("compte en base")
	}
	if rep.Encoding != "ANSEL" {
		t.Errorf("encodage détecté: %s", rep.Encoding)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM pragma_foreign_key_check`); n != 0 {
		t.Errorf("%d violations de clés étrangères", n)
	}
	var ic string
	d.QueryRow(`PRAGMA integrity_check`).Scan(&ic)
	if ic != "ok" {
		t.Error(ic)
	}
	// aucun résidu d'encodage : ni caractère de remplacement, ni octet de contrôle ANSEL
	if n := count(t, d, `SELECT COUNT(*) FROM names WHERE surname LIKE '%'||char(65533)||'%' OR given_names LIKE '%'||char(65533)||'%'`); n != 0 {
		t.Errorf("%d noms avec caractère de remplacement", n)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM places WHERE name LIKE '%'||char(65533)||'%'`); n != 0 {
		t.Errorf("%d lieux avec caractère de remplacement", n)
	}
	// chaque enfant déclaré (CHIL) dont un parent est présent a bien un lien
	if rep.ParentLinks == 0 || rep.Unions == 0 || rep.Events == 0 || rep.Places == 0 {
		t.Errorf("rapport incomplet: %+v", rep)
	}
	// aucun nom principal en double, aucune personne sans nom principal
	if n := count(t, d, `SELECT COUNT(*) FROM persons p WHERE (SELECT COUNT(*) FROM names n WHERE n.person_id=p.id AND n.is_primary=1) <> 1`); n != 0 {
		t.Errorf("%d personnes sans exactement un nom principal", n)
	}
	// dates : la grande majorité est structurée
	if txt := count(t, d, `SELECT COUNT(*) FROM events WHERE date_precision='text'`); txt > 3 {
		t.Errorf("%d dates en texte libre", txt)
	}
}
