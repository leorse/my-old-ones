package domain

import "sort"

// Parents donne le père et la mère (identifiants, "" si inconnu) d'une personne.
type Parents struct{ Father, Mother string }

// MaxSosaGenerations borne le calcul (protège aussi des données cycliques).
const MaxSosaGenerations = 30

// SosaNumbers numérote les ancêtres de root : 1 = root, 2n = père de n,
// 2n+1 = mère de n (spec Annexe A). Une personne présente plusieurs fois
// (implexe) reçoit plusieurs numéros, triés par ordre croissant.
func SosaNumbers(root string, parents func(id string) Parents, maxGen int) map[string][]int64 {
	if maxGen <= 0 || maxGen > MaxSosaGenerations {
		maxGen = MaxSosaGenerations
	}
	out := map[string][]int64{}
	type item struct {
		id  string
		n   int64
		gen int
	}
	queue := []item{{root, 1, 1}}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		out[it.id] = append(out[it.id], it.n)
		if it.gen >= maxGen {
			continue
		}
		p := parents(it.id)
		if p.Father != "" {
			queue = append(queue, item{p.Father, it.n * 2, it.gen + 1})
		}
		if p.Mother != "" {
			queue = append(queue, item{p.Mother, it.n*2 + 1, it.gen + 1})
		}
	}
	for _, ns := range out {
		sort.Slice(ns, func(i, j int) bool { return ns[i] < ns[j] })
	}
	return out
}

// Generation renvoie la génération (1 = racine) d'un numéro Sosa.
func Generation(n int64) int {
	g := 0
	for n > 0 {
		g++
		n >>= 1
	}
	return g
}
