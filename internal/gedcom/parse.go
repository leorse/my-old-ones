package gedcom

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Node est une ligne GEDCOM avec ses sous-lignes.
type Node struct {
	Level    int
	Xref     string // « @I1@ » sans les arobases : « I1 » (niveau 0 uniquement)
	Tag      string
	Value    string // CONC / CONT déjà appliqués
	Children []*Node
	Line     int // numéro de ligne dans le fichier (1-based)
}

// Child renvoie le premier enfant portant ce tag.
func (n *Node) Child(tag string) *Node {
	for _, c := range n.Children {
		if c.Tag == tag {
			return c
		}
	}
	return nil
}

// All renvoie tous les enfants portant ce tag.
func (n *Node) All(tag string) []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// Val renvoie la valeur du premier enfant portant ce tag ("" sinon).
func (n *Node) Val(tag string) string {
	if c := n.Child(tag); c != nil {
		return c.Value
	}
	return ""
}

// Pointer renvoie l'identifiant référencé par une valeur « @X1@ » ("" sinon).
func Pointer(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 2 && v[0] == '@' && v[len(v)-1] == '@' && !strings.Contains(v[1:len(v)-1], "@") {
		return v[1 : len(v)-1]
	}
	return ""
}

// Raw reconstruit le texte GEDCOM du sous-arbre, niveaux ramenés à base.
func (n *Node) Raw(base int) string {
	var sb strings.Builder
	var walk func(*Node, int)
	walk = func(x *Node, lvl int) {
		val := strings.ReplaceAll(x.Value, "\n", "\n"+strconv.Itoa(lvl+1)+" CONT ")
		fmt.Fprintf(&sb, "%d %s", lvl, x.Tag)
		if val != "" {
			sb.WriteString(" " + val)
		}
		sb.WriteString("\n")
		for _, c := range x.Children {
			walk(c, lvl+1)
		}
	}
	walk(n, base)
	return strings.TrimRight(sb.String(), "\n")
}

var reLine = regexp.MustCompile(`^\s*(\d+)\s+(?:@([^@\s]+)@\s+)?([^\s]+)(?:[ \t](.*))?$`)

// Parse construit l'arbre des enregistrements de niveau 0 à partir du texte décodé.
// Les anomalies non fatales sont renvoyées en avertissements.
func Parse(text string) (records []*Node, warnings []string) {
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	var stack []*Node // pile des ancêtres du dernier nœud
	var last *Node
	badLines := 0
	levelJumps := 0
	for i, raw := range lines {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		m := reLine.FindStringSubmatch(raw)
		if m == nil {
			// ligne sans niveau : continuation d'une valeur multiligne mal exportée
			badLines++
			if last != nil {
				last.Value += "\n" + strings.TrimSpace(raw)
			}
			continue
		}
		lvl, _ := strconv.Atoi(m[1])
		tag := strings.ToUpper(m[3])
		val := m[4]
		if strings.Contains(val, "@@") && Pointer(val) == "" { // arobase échappée (@@ -> @)
			val = strings.ReplaceAll(val, "@@", "@")
		}
		if lvl > 0 && (tag == "CONC" || tag == "CONT") && last != nil {
			if tag == "CONT" {
				last.Value += "\n"
			}
			last.Value += val
			continue
		}
		n := &Node{Level: lvl, Xref: m[2], Tag: tag, Value: val, Line: i + 1}
		if lvl == 0 {
			records = append(records, n)
			stack = stack[:0]
			stack = append(stack, n)
			last = n
			continue
		}
		if len(stack) == 0 {
			warnings = append(warnings, fmt.Sprintf("ligne %d : ligne de niveau %d avant tout enregistrement, ignorée", i+1, lvl))
			continue
		}
		// remonte jusqu'au parent de niveau lvl-1
		for len(stack) > 0 && stack[len(stack)-1].Level >= lvl {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			continue
		}
		parent := stack[len(stack)-1]
		if parent.Level != lvl-1 {
			levelJumps++
		}
		parent.Children = append(parent.Children, n)
		stack = append(stack, n)
		last = n
	}
	if badLines > 0 {
		warnings = append(warnings, fmt.Sprintf("%d ligne(s) sans niveau rattachée(s) à la valeur précédente", badLines))
	}
	if levelJumps > 0 {
		warnings = append(warnings, fmt.Sprintf("%d saut(s) de niveau corrigé(s)", levelJumps))
	}
	return records, warnings
}
