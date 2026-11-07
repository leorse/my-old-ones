// Package gedcom lit les fichiers GEDCOM 5.5.1 (et 7.0 pour l'essentiel) et les
// importe dans un projet : décodage des encodages, parseur de lignes,
// interprétation des dates et des lieux, import transactionnel avec rapport.
package gedcom

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/unicode/norm"
)

// Decoded est le texte d'un fichier GEDCOM après décodage.
type Decoded struct {
	Text     string
	Encoding string   // encodage effectivement utilisé
	Declared string   // valeur de HEAD.CHAR (peut différer)
	Warnings []string // anomalies de décodage
}

var reChar = regexp.MustCompile(`(?im)^\s*1\s+CHAR\s+([^\r\n]+)`)

// Decode détecte l'encodage (BOM, déclaration CHAR, heuristiques) et renvoie
// le texte en UTF-8. Gère UTF-8, UTF-16, ANSEL, ASCII, Windows-1252 / ISO-8859-1.
func Decode(raw []byte) (Decoded, error) {
	var d Decoded
	switch {
	case bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}):
		d.Encoding = "UTF-8"
		d.Text = string(raw[3:])
		d.Declared = declared(d.Text)
		return d.finish(), nil
	case bytes.HasPrefix(raw, []byte{0xFF, 0xFE}), bytes.HasPrefix(raw, []byte{0xFE, 0xFF}):
		return decodeWith(raw, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder(), "UTF-16")
	}
	// UTF-16 sans BOM : beaucoup d'octets nuls dans les premiers caractères
	if len(raw) >= 8 && (raw[1] == 0 && raw[3] == 0 || raw[0] == 0 && raw[2] == 0) {
		order := unicode.LittleEndian
		if raw[0] == 0 {
			order = unicode.BigEndian
		}
		return decodeWith(raw, unicode.UTF16(order, unicode.IgnoreBOM).NewDecoder(), "UTF-16")
	}

	head := raw
	if len(head) > 4096 {
		head = head[:4096]
	}
	if m := reChar.FindSubmatch(head); m != nil {
		d.Declared = strings.ToUpper(strings.TrimSpace(string(m[1])))
	}
	isASCII := true
	for _, b := range raw {
		if b > 127 {
			isASCII = false
			break
		}
	}
	validUTF8 := utf8.Valid(raw)

	switch d.Declared {
	case "UTF-8", "UTF8":
		if validUTF8 {
			d.Encoding, d.Text = "UTF-8", string(raw)
			break
		}
		d.Warnings = append(d.Warnings, "le fichier déclare UTF-8 mais n'est pas de l'UTF-8 valide : lu comme Windows-1252")
		return decodeWith(raw, charmap.Windows1252.NewDecoder(), "Windows-1252", d.Warnings...)
	case "ANSEL":
		if validUTF8 && !isASCII {
			d.Warnings = append(d.Warnings, "le fichier déclare ANSEL mais contient de l'UTF-8 valide : lu comme UTF-8")
			d.Encoding, d.Text = "UTF-8", string(raw)
			break
		}
		d.Encoding = "ANSEL"
		d.Text = decodeANSEL(raw, &d.Warnings)
	case "ASCII", "":
		switch {
		case isASCII || validUTF8:
			d.Encoding, d.Text = "UTF-8", string(raw)
		default:
			d.Warnings = append(d.Warnings, "caractères non ASCII dans un fichier ASCII : lu comme Windows-1252")
			return decodeWith(raw, charmap.Windows1252.NewDecoder(), "Windows-1252", d.Warnings...)
		}
	case "ANSI", "WINDOWS-1252", "CP1252", "WINDOWS", "ANSI/WINDOWS":
		if validUTF8 && !isASCII {
			d.Encoding, d.Text = "UTF-8", string(raw)
			break
		}
		return decodeWith(raw, charmap.Windows1252.NewDecoder(), "Windows-1252")
	case "ISO-8859-1", "ISO8859-1", "LATIN1", "ISO_8859-1", "ISO-8859":
		if validUTF8 && !isASCII {
			d.Encoding, d.Text = "UTF-8", string(raw)
			break
		}
		return decodeWith(raw, charmap.ISO8859_1.NewDecoder(), "ISO-8859-1")
	case "IBMPC", "IBM", "CP437", "OEM":
		return decodeWith(raw, charmap.CodePage437.NewDecoder(), "CP437")
	case "MACINTOSH", "MAC":
		return decodeWith(raw, charmap.Macintosh.NewDecoder(), "Macintosh")
	case "UNICODE":
		return decodeWith(raw, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder(), "UTF-16")
	default:
		if validUTF8 {
			d.Encoding, d.Text = "UTF-8", string(raw)
		} else {
			d.Warnings = append(d.Warnings, fmt.Sprintf("encodage %q inconnu : lu comme Windows-1252", d.Declared))
			return decodeWith(raw, charmap.Windows1252.NewDecoder(), "Windows-1252", d.Warnings...)
		}
	}
	return d.finish(), nil
}

func declared(text string) string {
	if m := reChar.FindStringSubmatch(text); m != nil {
		return strings.ToUpper(strings.TrimSpace(m[1]))
	}
	return ""
}

func decodeWith(raw []byte, dec *encoding.Decoder, name string, warnings ...string) (Decoded, error) {
	out, err := dec.Bytes(raw)
	if err != nil {
		return Decoded{}, fmt.Errorf("décodage %s: %w", name, err)
	}
	d := Decoded{Text: string(out), Encoding: name, Warnings: warnings}
	d.Declared = declared(d.Text)
	return d.finish(), nil
}

func (d Decoded) finish() Decoded {
	d.Text = strings.TrimPrefix(d.Text, string(rune(0xFEFF)))
	if d.Declared == "" {
		d.Declared = declared(d.Text)
	}
	return d
}

// --- ANSEL ------------------------------------------------------------------

// Signes diacritiques ANSEL : ils PRÉCÈDENT la lettre de base (contrairement
// à Unicode, où ils la suivent).
var anselCombining = map[byte]rune{
	0xE0: '̉', 0xE1: '̀', 0xE2: '́', 0xE3: '̂', 0xE4: '̃',
	0xE5: '̄', 0xE6: '̆', 0xE7: '̇', 0xE8: '̈', 0xE9: '̌',
	0xEA: '̊', 0xEB: '͡', 0xEC: '͡', 0xED: '̕', 0xEE: '̋',
	0xEF: '̐', 0xF0: '̧', 0xF1: '̨', 0xF2: '̣', 0xF3: '̤',
	0xF4: '̥', 0xF5: '̳', 0xF6: '̲', 0xF7: '̦', 0xF8: '̜',
	0xF9: '̮', 0xFA: '͠', 0xFE: '̓',
}

var anselSpacing = map[byte]rune{
	0xA1: 'Ł', 0xA2: 'Ø', 0xA3: 'Đ', 0xA4: 'Þ', 0xA5: 'Æ', 0xA6: 'Œ', 0xA7: 'ʹ',
	0xA8: '·', 0xA9: '♭', 0xAA: '®', 0xAB: '±', 0xAC: 'Ơ', 0xAD: 'Ư', 0xAE: 'ʼ',
	0xB0: 'ʻ', 0xB1: 'ł', 0xB2: 'ø', 0xB3: 'đ', 0xB4: 'þ', 0xB5: 'æ', 0xB6: 'œ',
	0xB7: 'ʺ', 0xB8: 'ı', 0xB9: '£', 0xBA: 'ð', 0xBC: 'ơ', 0xBD: 'ư',
	0xC0: '°', 0xC1: 'ℓ', 0xC2: '℗', 0xC3: '©', 0xC4: '♯', 0xC5: '¿', 0xC6: '¡',
	0xCF: 'ß',
}

func decodeANSEL(raw []byte, warnings *[]string) string {
	var sb strings.Builder
	var pending []rune
	unknown := 0
	flush := func(base rune) {
		sb.WriteRune(base)
		for _, m := range pending {
			sb.WriteRune(m)
		}
		pending = pending[:0]
	}
	for _, b := range raw {
		switch {
		case b < 0x80:
			if len(pending) > 0 && b >= 0x20 {
				flush(rune(b))
			} else {
				if len(pending) > 0 { // marque orpheline avant un saut de ligne
					pending = pending[:0]
				}
				sb.WriteByte(b)
			}
		case anselCombining[b] != 0:
			pending = append(pending, anselCombining[b])
		case anselSpacing[b] != 0:
			flush(anselSpacing[b])
		case b == 0x88, b == 0x89, b == 0x8D, b == 0x8E: // non-tri, joint : ignorés
		default:
			unknown++
			flush('�')
		}
	}
	if unknown > 0 {
		*warnings = append(*warnings, fmt.Sprintf("%d caractère(s) ANSEL non reconnu(s) remplacé(s)", unknown))
	}
	return norm.NFC.String(sb.String())
}
