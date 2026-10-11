// Package bankimport parses bank statements (OFX and CSV) and suggests what to do with each line.
// It has no database or HTTP code: everything here is a pure function over bytes and values.
package bankimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxFileBytes is the largest statement file accepted; a month of statements is a few tens of KB.
const MaxFileBytes = 5 << 20

// NoDescription is shown (to the user, in pt-BR) for statement lines that carry no text.
const NoDescription = "Sem descrição"

// Entry is one line of a statement. AmountCents is signed as in the file: negative is money out.
// Date is the bank's calendar date (YYYY-MM-DD), taken as written, with no time zone conversion.
type Entry struct {
	ExternalID  string
	Date        string
	AmountCents int64
	Description string
}

var cp1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š', 0x8B: '‹',
	0x8C: 'Œ', 0x8E: 'Ž', 0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—', 0x98: '˜', 0x99: '™',
	0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}

// decodeText turns the bytes of a statement into a string. Brazilian banks often export Windows-1252 (CHARSET:1252),
// which is not valid UTF-8, so anything that is not valid UTF-8 is read as Windows-1252.
func decodeText(raw []byte) string {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 byte order mark
	if utf8.Valid(raw) {
		return string(raw)
	}
	var b strings.Builder
	for _, c := range raw {
		if r, ok := cp1252[c]; ok {
			b.WriteRune(r)
		} else {
			b.WriteRune(rune(c)) // 0x00-0x7F and 0xA0-0xFF map to the same code point in Latin-1
		}
	}
	return b.String()
}

// tidy collapses runs of whitespace and trims the ends.
func tidy(s string) string { return strings.Join(strings.Fields(s), " ") }

// fallbackID builds a stable id for a line that has none. nth counts earlier identical lines in the same
// file, so two real purchases of the same value on the same day stay distinct but re-parsing gives the same ids.
func fallbackID(date string, cents int64, description string, nth int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s|%d", date, cents, description, nth)))
	return "h-" + hex.EncodeToString(sum[:8])
}

// uniqueID keeps ids unique inside one file (some banks reuse a FITID), deterministically.
func uniqueID(seen map[string]int, id string) string {
	seen[id]++
	if n := seen[id]; n > 1 {
		return fmt.Sprintf("%s~%d", id, n)
	}
	return id
}
