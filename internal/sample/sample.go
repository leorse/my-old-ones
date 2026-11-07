// Package sample génère un projet d'exemple : une grande famille bretonne
// fictive sur sept générations (1780 → années 2000), déterministe à graine égale.
// Toutes les données sont inventées.
package sample

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"moo/internal/domain"
	"moo/internal/repo"
)

const nowYear = 2025

// Options règle la génération.
type Options struct {
	Seed      int64 // 0 = graine par défaut
	MaxPeople int   // plafond approximatif de personnes (0 = 800)
}

// Result résume ce qui a été créé.
type Result struct {
	People  int
	Unions  int
	Events  int
	Places  int
	RootID  string
	RootGen int
}

type person struct {
	id      string
	sex     string // M | F
	surname string
	given   []string
	birth   int
	bMonth  int
	bDay    int
	death   int // 0 = vivant
	place   string
	gen     int
}

type gen struct {
	ctx    context.Context
	s      *repo.Store
	r      *rand.Rand
	max    int
	people int
	places []string // identifiants de communes
	err    error
	adopts int
	byGen  map[int][]*person
}

// Generate remplit un projet vide. L'historique annuler/rétablir est vidé à la fin.
func Generate(ctx context.Context, s *repo.Store, opt Options) (Result, error) {
	seed := opt.Seed
	if seed == 0 {
		seed = 1789
	}
	max := opt.MaxPeople
	if max <= 0 {
		max = 800
	}
	g := &gen{ctx: ctx, s: s, r: rand.New(rand.NewSource(seed)), max: max, byGen: map[int][]*person{}}
	g.createPlaces()

	// Couple fondateur : lui né vers 1780, elle vers 1784.
	father := g.founder("M", "Le Gall", 1780)
	mother := g.founder("F", "Kerjean", 1784)
	g.must(g.s.UpdatePerson(ctx, father.id, repo.PersonPatch{
		NotesMD: ptr("Ancêtre fondateur de la famille Le Gall de Saint-Malo, marin puis cultivateur.\n\nLes registres paroissiaux de l'époque mentionnent plusieurs enfants.")}))
	queue := []*person{}
	g.marry(father, mother, 1804, &queue)

	for len(queue) > 0 && g.err == nil && g.people < g.max {
		p := queue[0]
		queue = queue[1:]
		g.family(p, &queue)
	}
	if g.err != nil {
		return Result{}, g.err
	}

	// Personne racine : un descendant récent à l'ascendance la plus riche.
	rootID, rootGen := g.pickRoot()
	if rootID != "" {
		g.must(g.s.SetRoot(ctx, rootID))
	}
	// Le projet d'exemple démarre sans historique.
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM history_entries`); err != nil {
		return Result{}, err
	}
	res := Result{RootID: rootID, RootGen: rootGen}
	for table, dst := range map[string]*int{"persons": &res.People, "unions": &res.Unions, "events": &res.Events, "places": &res.Places} {
		if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(dst); err != nil {
			return res, err
		}
	}
	return res, g.err
}

func ptr[T any](v T) *T { return &v }

func (g *gen) must(err error) {
	if err != nil && g.err == nil {
		g.err = err
	}
}

// --- lieux -----------------------------------------------------------------

type commune struct {
	name, insee, dept string
	lat, lon          float64
}

var departments = map[string]string{"35": "Ille-et-Vilaine", "56": "Morbihan", "29": "Finistère", "44": "Loire-Atlantique", "22": "Côtes-d'Armor"}

var communes = []commune{
	{"Saint-Malo", "35288", "35", 48.649, -2.026}, {"Rennes", "35238", "35", 48.117, -1.678},
	{"Cancale", "35049", "35", 48.676, -1.852}, {"Dinan", "22050", "22", 48.455, -2.050},
	{"Vannes", "56260", "56", 47.658, -2.760}, {"Lorient", "56121", "56", 47.748, -3.366},
	{"Quimper", "29232", "29", 47.996, -4.102}, {"Brest", "29019", "29", 48.390, -4.486},
	{"Morlaix", "29151", "29", 48.577, -3.828}, {"Nantes", "44109", "44", 47.218, -1.554},
	{"Saint-Brieuc", "22278", "22", 48.514, -2.760}, {"Paimpol", "22162", "22", 48.777, -3.047},
}

func (g *gen) createPlaces() {
	ctx := g.ctx
	fr, err := g.s.CreatePlace(ctx, repo.PlaceInput{Name: "France", Type: "country"})
	g.must(err)
	reg, err := g.s.CreatePlace(ctx, repo.PlaceInput{Name: "Bretagne", Type: "region", ParentID: fr})
	g.must(err)
	deptIDs := map[string]string{}
	for _, code := range []string{"22", "29", "35", "56", "44"} {
		parent := reg
		if code == "44" {
			var perr error
			parent, perr = g.s.CreatePlace(ctx, repo.PlaceInput{Name: "Pays de la Loire", Type: "region", ParentID: fr})
			g.must(perr)
		}
		id, perr := g.s.CreatePlace(ctx, repo.PlaceInput{Name: departments[code], Type: "department", ParentID: parent})
		g.must(perr)
		deptIDs[code] = id
	}
	for _, c := range communes {
		lat, lon := c.lat, c.lon
		id, err := g.s.CreatePlace(ctx, repo.PlaceInput{Name: c.name, Type: "commune", ParentID: deptIDs[c.dept],
			CodeINSEE: c.insee, Latitude: &lat, Longitude: &lon})
		g.must(err)
		g.places = append(g.places, id)
	}
}

func (g *gen) randomPlace(near string) string {
	if near != "" && g.r.Float64() < 0.6 {
		return near
	}
	return g.places[g.r.Intn(len(g.places))]
}

// --- noms ------------------------------------------------------------------

var surnames = []string{"Le Goff", "Le Bihan", "Morvan", "Guillou", "Cadiou", "Le Roux", "Bodin", "Tanguy",
	"Kermarrec", "Rolland", "Lemoine", "Prigent", "Quéré", "Thomas", "Garnier", "Le Floch", "Hamon", "Jégou",
	"Le Corre", "Lozach", "Perrot", "Moal", "Salaün", "Riou", "Cariou", "Josse", "Autret", "Bellec", "Calvez",
	"Pichon", "Collet", "Marchand", "Kerjean", "Le Gall", "Le Gac", "Mével", "Nicolas", "Simon", "Lucas"}

var maleOld = []string{"Jean", "Pierre", "Yves", "François", "Joseph", "Louis", "Guillaume", "Mathurin", "Julien", "René", "Olivier", "Hervé", "Jacques", "Vincent", "Alain"}
var femaleOld = []string{"Marie", "Anne", "Jeanne", "Françoise", "Marguerite", "Perrine", "Catherine", "Louise", "Yvonne", "Hélène", "Corentine", "Jacquette", "Mathurine", "Renée", "Julienne"}
var malePrewar = []string{"Jean", "Pierre", "Joseph", "Louis", "Marcel", "Henri", "André", "Paul", "Yves", "Émile", "Léon", "Eugène", "Auguste", "Maurice", "Georges"}
var femalePrewar = []string{"Marie", "Jeanne", "Marguerite", "Yvonne", "Anne", "Louise", "Germaine", "Marcelle", "Suzanne", "Simone", "Odette", "Madeleine", "Thérèse", "Léonie", "Augustine"}
var malePostwar = []string{"Michel", "Alain", "Gérard", "Bernard", "Daniel", "Claude", "Jacques", "Patrick", "Gilles", "Christian", "Philippe", "Thierry", "Dominique", "Bruno", "Serge"}
var femalePostwar = []string{"Monique", "Nicole", "Jacqueline", "Annie", "Christine", "Martine", "Françoise", "Sylvie", "Catherine", "Nathalie", "Isabelle", "Brigitte", "Danielle", "Chantal", "Michèle"}
var maleModern = []string{"Nicolas", "Julien", "Thomas", "Maxime", "Antoine", "Alexandre", "Romain", "Kévin", "Lucas", "Hugo", "Théo", "Louis", "Gabriel", "Arthur", "Mathis"}
var femaleModern = []string{"Julie", "Camille", "Marie", "Laura", "Sarah", "Léa", "Emma", "Manon", "Chloé", "Pauline", "Clara", "Inès", "Jade", "Louise", "Anaïs"}

func (g *gen) givens(sex string, birth int) []string {
	var pool []string
	switch {
	case birth < 1860:
		pool = pick(sex, maleOld, femaleOld)
	case birth < 1935:
		pool = pick(sex, malePrewar, femalePrewar)
	case birth < 1975:
		pool = pick(sex, malePostwar, femalePostwar)
	default:
		pool = pick(sex, maleModern, femaleModern)
	}
	first := pool[g.r.Intn(len(pool))]
	if birth < 1900 && g.r.Float64() < 0.3 { // prénoms composés d'époque
		first += "-" + pool[g.r.Intn(len(pool))]
	}
	names := []string{first}
	if birth >= 1860 && g.r.Float64() < 0.5 {
		names = append(names, pool[g.r.Intn(len(pool))])
	}
	return names
}

func pick(sex string, m, f []string) []string {
	if sex == "M" {
		return m
	}
	return f
}

// --- métiers ---------------------------------------------------------------

var jobsMaleOld = []string{"laboureur", "journalier", "marin", "pêcheur", "cultivateur", "tisserand", "forgeron", "meunier", "sabotier", "charpentier", "maçon", "boulanger", "tailleur de pierre"}
var jobsFemaleOld = []string{"ménagère", "couturière", "lingère", "cultivatrice", "fileuse", "sage-femme", "aubergiste"}
var jobsMaleNew = []string{"instituteur", "notaire", "médecin", "ouvrier d'usine", "employé SNCF", "mécanicien", "ingénieur", "commerçant", "enseignant", "marin-pêcheur", "gendarme", "électricien", "comptable"}
var jobsFemaleNew = []string{"institutrice", "commerçante", "infirmière", "employée de bureau", "enseignante", "ingénieure", "secrétaire", "pharmacienne", "couturière", "comptable"}

func (g *gen) job(p *person) string {
	var pool []string
	switch {
	case p.sex == "M" && p.birth < 1880:
		pool = jobsMaleOld
	case p.sex == "F" && p.birth < 1880:
		pool = jobsFemaleOld
	case p.sex == "M":
		pool = jobsMaleNew
	default:
		pool = jobsFemaleNew
	}
	return pool[g.r.Intn(len(pool))]
}

// --- dates -----------------------------------------------------------------

// dateText produit une date plausible pour l'année donnée, parfois imprécise
// ou en calendrier républicain (1793–1805).
func (g *gen) dateText(year, month, day int, allowFuzzy bool) string {
	if allowFuzzy && year < 1830 {
		switch x := g.r.Float64(); {
		case x < 0.12:
			return fmt.Sprintf("vers %d", year)
		case x < 0.15:
			return fmt.Sprintf("entre %d et %d", year-1, year+1)
		case x < 0.20:
			return fmt.Sprint(year)
		}
	}
	if year >= 1793 && year <= 1805 && g.r.Float64() < 0.5 {
		if ry, rm, rd, err := domain.GregorianToRepublican(year, month, day); err == nil && rm <= 12 {
			return fmt.Sprintf("%d %s an %s", rd, domain.RepublicanMonths[rm], domain.IntToRoman(ry))
		}
	}
	return fmt.Sprintf("%d/%d/%d", day, month, year)
}

func (g *gen) lifespan(birth int) int {
	r := g.r.Float64()
	var age int
	childMort := 0.14
	if birth >= 1920 {
		childMort = 0.04
	}
	if birth >= 1950 {
		childMort = 0.005
	}
	switch {
	case r < childMort:
		age = g.r.Intn(6)
	case r < childMort+0.06:
		age = 6 + g.r.Intn(30)
	default:
		age = 55 + int(g.r.NormFloat64()*13)
		if age < 36 {
			age = 36 + g.r.Intn(10)
		}
		if age > 101 {
			age = 101
		}
	}
	if birth+age > nowYear {
		return 0
	}
	return birth + age
}

// lifespanAtLeast tire une durée de vie telle que le décès soit ≥ minDeath (ou vivant).
func (g *gen) lifespanAtLeast(birth, minDeath int) int {
	for i := 0; i < 12; i++ {
		d := g.lifespan(birth)
		if d == 0 || d >= minDeath {
			return d
		}
	}
	if birth+80 > nowYear {
		return 0
	}
	return max(minDeath, birth+60)
}

// --- création de personnes -------------------------------------------------

func (g *gen) finish(p *person) {
	if g.err != nil {
		return
	}
	g.people++
	p.bMonth, p.bDay = 1+g.r.Intn(12), 1+g.r.Intn(27)
	ctx := g.ctx
	ev := func(typ, date, place, desc string) {
		_, err := g.s.AddEvent(ctx, repo.EventInput{Type: typ, PersonID: p.id, Date: date, PlaceID: place, Description: desc})
		g.must(err)
	}
	ev("birth", g.dateText(p.birth, p.bMonth, p.bDay, true), p.place, "")
	if p.death > 0 {
		dm, dd := p.bMonth, 1+g.r.Intn(27)
		if p.death > p.birth {
			dm = 1 + g.r.Intn(12)
		} else {
			dm = p.bMonth + g.r.Intn(13-p.bMonth)
			dd = p.bDay + 1
			if dm > p.bMonth {
				dd = 1 + g.r.Intn(27)
			}
		}
		ev("death", g.dateText(p.death, dm, dd, p.death-p.birth > 5), g.randomPlace(p.place), "")
		if p.birth < 1950 && g.r.Float64() < 0.5 {
			ev("burial", g.dateText(p.death, dm, min(dd+2, 28), false), g.randomPlace(p.place), "")
		}
	}
	if p.birth < 2000 && (p.death == 0 || p.death-p.birth >= 18) {
		ey := p.birth + 20 + g.r.Intn(10)
		if ey <= nowYear && (p.death == 0 || ey < p.death) {
			desc := g.job(p)
			if g.r.Float64() < 0.4 {
				ev("occupation", fmt.Sprintf("vers %d", ey), g.randomPlace(p.place), desc)
			} else {
				ev("occupation", "", "", desc)
			}
			if g.r.Float64() < 0.2 {
				ry := ey + 3 + g.r.Intn(15)
				if p.death == 0 && ry <= nowYear || p.death > 0 && ry < p.death {
					ev("residence", fmt.Sprint(ry), g.randomPlace(p.place), "")
				}
			}
		}
	}
	if p.death == 0 && p.birth < 1925 { // plus de cent ans : improbable, filet de sécurité
		g.must(fmt.Errorf("personne centenaire vivante: %d", p.birth))
	}
	if p.death == 0 {
		g.must(g.s.UpdatePerson(ctx, p.id, repo.PersonPatch{Living: ptr("living")}))
	}
	if g.r.Float64() < 0.03 {
		g.must(g.s.UpdatePerson(ctx, p.id, repo.PersonPatch{IsFavorite: ptr(true)}))
	}
}

func (g *gen) newPerson(p *person, create func(np repo.NewPerson) (string, error)) *person {
	if g.err != nil {
		return p
	}
	p.given = g.givens(p.sex, p.birth)
	id, err := create(repo.NewPerson{Surname: p.surname, GivenNames: p.given, Sex: p.sex})
	g.must(err)
	p.id = id
	g.finish(p)
	g.byGen[p.gen] = append(g.byGen[p.gen], p)
	return p
}

func (g *gen) founder(sex, surname string, birth int) *person {
	p := &person{sex: sex, surname: surname, birth: birth, place: g.places[0], gen: 0}
	p.death = g.lifespan(birth)
	return g.newPerson(p, func(np repo.NewPerson) (string, error) { return g.s.CreatePerson(g.ctx, np) })
}

func (g *gen) unionBetween(a, b string) string {
	var id string
	err := g.s.DB.QueryRowContext(g.ctx, `SELECT a.union_id FROM union_partners a JOIN union_partners b
		ON a.union_id = b.union_id WHERE a.person_id = ? AND b.person_id = ? LIMIT 1`, a, b).Scan(&id)
	g.must(err)
	return id
}

// outsider crée le conjoint d'un descendant, avec ses parents et sa fratrie.
func (g *gen) outsider(of *person, marryYear int) *person {
	sex := "M"
	if of.sex == "M" {
		sex = "F"
	}
	birth := of.birth + g.r.Intn(9) - 4
	if marryYear-birth < 18 {
		birth = marryYear - 18 - g.r.Intn(4)
	}
	sp := &person{sex: sex, surname: surnames[g.r.Intn(len(surnames))], birth: birth,
		place: g.randomPlace(of.place), gen: of.gen}
	sp.death = g.lifespanAtLeast(birth, marryYear+1)
	g.newPerson(sp, func(np repo.NewPerson) (string, error) { return g.s.CreatePerson(g.ctx, np) })

	if g.r.Float64() < 0.55 && birth < 1990 && g.people < g.max {
		g.parentsOf(sp, 1)
	}
	return sp
}

// parentsOf ajoute père, mère et quelques frères et sœurs à une personne.
func (g *gen) parentsOf(c *person, depth int) {
	fb := c.birth - 24 - g.r.Intn(9)
	mb := c.birth - 21 - g.r.Intn(9)
	fa := &person{sex: "M", surname: c.surname, birth: fb, place: c.place, gen: c.gen - 1}
	fa.death = g.lifespanAtLeast(fb, c.birth-1)
	g.newPerson(fa, func(np repo.NewPerson) (string, error) {
		return g.s.CreateRelative(g.ctx, c.id, repo.RelFather, np, repo.RelativeOpts{})
	})
	mo := &person{sex: "F", surname: surnames[g.r.Intn(len(surnames))], birth: mb, place: c.place, gen: c.gen - 1}
	mo.death = g.lifespanAtLeast(mb, c.birth)
	g.newPerson(mo, func(np repo.NewPerson) (string, error) {
		return g.s.CreateRelative(g.ctx, c.id, repo.RelMother, np, repo.RelativeOpts{})
	})
	if g.err != nil {
		return
	}
	u := g.unionBetween(fa.id, mo.id)
	my := max(fb, mb) + 18 + g.r.Intn(4)
	if my >= c.birth {
		my = c.birth - 1
	}
	g.marriageEvent(u, my, fa.place)
	// l'ascendance du conjoint se prolonge parfois sur plusieurs générations
	for _, anc := range []*person{fa, mo} {
		if depth < 4 && anc.birth < 1900 && g.people < g.max && g.r.Float64() < 0.7/float64(depth) {
			g.parentsOf(anc, depth+1)
		}
	}
	// fratrie (feuilles)
	for i, n := 0, g.r.Intn(4); i < n; i++ {
		by := c.birth + g.r.Intn(11) - 5
		if by < mb+17 || (mo.death > 0 && by > mo.death) || (fa.death > 0 && by > fa.death+1) || by > nowYear || by == c.birth {
			continue
		}
		sib := &person{sex: []string{"M", "F"}[g.r.Intn(2)], surname: fa.surname, birth: by, place: c.place, gen: c.gen}
		sib.death = g.lifespanAtLeast(by, by)
		g.newPerson(sib, func(np repo.NewPerson) (string, error) {
			return g.s.CreateRelative(g.ctx, c.id, repo.RelSibling, np, repo.RelativeOpts{})
		})
	}
}

func (g *gen) marriageEvent(unionID string, year int, place string) {
	_, err := g.s.AddEvent(g.ctx, repo.EventInput{Type: "marriage", UnionID: unionID,
		Date: g.dateText(year, 1+g.r.Intn(12), 1+g.r.Intn(27), true), PlaceID: g.randomPlace(place)})
	g.must(err)
}

// marry unit deux personnes et enfile les enfants à traiter.
func (g *gen) marry(a, b *person, year int, queue *[]*person) {
	if g.err != nil {
		return
	}
	u, err := g.s.CreateUnion(g.ctx, "marriage", []string{a.id, b.id})
	g.must(err)
	g.marriageEvent(u, year, a.place)
	g.children(a, b, u, year, queue)
}

func (g *gen) children(a, b *person, unionID string, marryYear int, queue *[]*person) {
	father, mother := a, b
	if father.sex != "M" {
		father, mother = b, a
	}
	gnum := a.gen
	if b.gen > gnum {
		gnum = b.gen
	}
	lo, hi := childRange(gnum)
	n := lo + g.r.Intn(hi-lo+1)
	year := marryYear + 1 + g.r.Intn(2)
	for i := 0; i < n && g.err == nil && g.people < g.max; i++ {
		if year > nowYear || year-mother.birth > 42 {
			break
		}
		if (mother.death > 0 && year > mother.death) || (father.death > 0 && year > father.death+1) {
			break
		}
		sex := []string{"M", "F"}[g.r.Intn(2)]
		c := &person{sex: sex, surname: father.surname, birth: year, place: g.randomPlace(father.place), gen: gnum + 1}
		c.death = g.lifespan(year)
		g.newPerson(c, func(np repo.NewPerson) (string, error) {
			return g.s.CreateRelative(g.ctx, mother.id, repo.RelChild, np, repo.RelativeOpts{UnionID: unionID})
		})
		*queue = append(*queue, c)
		year += 1 + g.r.Intn(3)
	}
	// adoption occasionnelle
	if g.adopts < 3 && gnum >= 2 && g.r.Float64() < 0.05 && g.people < g.max {
		by := marryYear + 4 + g.r.Intn(8)
		if by <= nowYear && (father.death == 0 || by < father.death) && (mother.death == 0 || by < mother.death) {
			ad := &person{sex: []string{"M", "F"}[g.r.Intn(2)], surname: surnames[g.r.Intn(len(surnames))],
				birth: by, place: g.randomPlace(father.place), gen: gnum + 1}
			ad.death = g.lifespan(by)
			g.newPerson(ad, func(np repo.NewPerson) (string, error) { return g.s.CreatePerson(g.ctx, np) })
			if g.err == nil {
				_, e1 := g.s.LinkParent(g.ctx, ad.id, father.id, "father", "adoptive")
				_, e2 := g.s.LinkParent(g.ctx, ad.id, mother.id, "mother", "adoptive")
				g.must(e1)
				g.must(e2)
				g.adopts++
			}
		}
	}
}

func childRange(g int) (int, int) {
	switch g {
	case 0:
		return 4, 6
	case 1:
		return 3, 5
	case 2:
		return 2, 4
	case 3:
		return 2, 3
	case 4:
		return 1, 3
	case 5:
		return 1, 3
	}
	return 0, 2
}

// family traite un descendant : mariage(s), nom marital, enfants.
func (g *gen) family(p *person, queue *[]*person) {
	if g.err != nil || g.people >= g.max {
		return
	}
	prob := 0.85
	if p.birth > 1965 {
		prob = 0.6
	}
	if g.r.Float64() > prob {
		return
	}
	age := 22 + g.r.Intn(11)
	if p.sex == "F" {
		age = 19 + g.r.Intn(10)
	}
	year := p.birth + age
	if year > nowYear || (p.death > 0 && p.death <= year) {
		return
	}
	sp := g.outsider(p, year)
	g.link(p, sp, year, queue)

	// remariage éventuel après veuvage
	if sp.death > 0 && sp.death < nowYear-10 && (p.death == 0 || p.death > sp.death+3) && g.r.Float64() < 0.4 && g.people < g.max {
		ry := sp.death + 2 + g.r.Intn(5)
		if ry <= nowYear && (p.death == 0 || ry < p.death) {
			sp2 := g.outsider(p, ry)
			g.link(p, sp2, ry, queue)
		}
	}
}

// link crée l'union p–sp (avec nom marital pour l'épouse) et leurs enfants.
func (g *gen) link(p, sp *person, year int, queue *[]*person) {
	if g.err != nil {
		return
	}
	husband, wife := p, sp
	if p.sex == "F" {
		husband, wife = sp, p
	}
	_, err := g.s.AddName(g.ctx, wife.id, repo.NameInput{Type: "marital", Surname: husband.surname, GivenNames: wife.given})
	g.must(err)
	u, err := g.s.CreateUnion(g.ctx, "marriage", []string{husband.id, wife.id})
	g.must(err)
	g.marriageEvent(u, year, husband.place)
	// union terminée : décès d'un conjoint, ou divorce (rare, après 1884)
	if (husband.death > 0 && husband.death <= nowYear) || (wife.death > 0 && wife.death <= nowYear) {
		g.must(g.s.UpdateUnion(g.ctx, u, repo.UnionPatch{Status: ptr("ended")}))
	} else if year > 1900 && g.r.Float64() < 0.08 {
		dy := year + 5 + g.r.Intn(15)
		if dy <= nowYear {
			_, err := g.s.AddEvent(g.ctx, repo.EventInput{Type: "divorce", UnionID: u, Date: fmt.Sprint(dy)})
			g.must(err)
			g.must(g.s.UpdateUnion(g.ctx, u, repo.UnionPatch{Status: ptr("ended")}))
		}
	}
	g.children(p, sp, u, year, queue)
}

// pickRoot choisit, parmi les descendants récents, celui qui a le plus d'ancêtres numérotés.
func (g *gen) pickRoot() (string, int) {
	best, bestN, bestGen := "", -1, 0
	for gn := 12; gn >= 0 && best == ""; gn-- {
		for _, p := range g.byGen[gn] {
			nums, err := g.s.SosaNumbers(g.ctx, p.id, 0)
			if err != nil {
				g.must(err)
				return "", 0
			}
			score := len(nums)
			if p.death == 0 {
				score += 1000 // on préfère une personne vivante
			}
			if score > bestN {
				best, bestN, bestGen = p.id, score, gn
			}
		}
	}
	return best, bestGen
}

var _ = strings.TrimSpace
