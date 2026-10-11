package bankimport

import (
	"errors"
	"strconv"
	"strings"
)

var errBadAmount = errors.New("invalid amount")

// ParseAmount reads a money amount as written in a statement and returns signed whole cents, never going through floats.
//
// It accepts "48.90", "48,90", "1.234,56", "1,234.56", "R$ -48,90", "-R$ 48,90" and "(48,90)" (negative).
// With a single separator and exactly three digits after it ("1.234") the separator is a thousands group, as in Brazil;
// with one or two digits it is the decimal separator. More than two decimals is an error rather than a guess.
func ParseAmount(in string) (int64, error) {
	s := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(in, "R$", ""), " ", " "))
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg, s = true, strings.TrimSpace(s[1:len(s)-1])
	}
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = neg != (s[0] == '-')
		s = strings.TrimSpace(s[1:])
	}
	if s == "" || strings.ContainsAny(s, " \t\r\n+-()") || strings.Trim(s, "0123456789.,") != "" {
		return 0, errBadAmount
	}

	dot, comma := strings.LastIndexByte(s, '.'), strings.LastIndexByte(s, ',')
	intPart, frac := s, ""
	var thousandsSep byte
	switch {
	case dot >= 0 && comma >= 0: // both present: the later one is the decimal separator
		d := max(dot, comma)
		intPart, frac = s[:d], s[d+1:]
		thousandsSep = s[min(dot, comma)]
		if strings.Count(s, string(s[d])) != 1 {
			return 0, errBadAmount
		}
	case dot >= 0 || comma >= 0:
		sep := byte('.')
		idx := dot
		if comma >= 0 {
			sep, idx = ',', comma
		}
		after := len(s) - idx - 1
		switch {
		case strings.Count(s, string(sep)) > 1: // 1.234.567: only thousands groups
			thousandsSep = sep
		case after == 3 && sep == '.': // 1.234
			thousandsSep = sep
		case after == 0 || after > 2:
			return 0, errBadAmount
		default:
			intPart, frac = s[:idx], s[idx+1:]
		}
	}
	if thousandsSep != 0 {
		var ok bool
		if intPart, ok = ungroup(intPart, thousandsSep); !ok {
			return 0, errBadAmount
		}
	}
	if len(frac) > 2 || intPart == "" || strings.Trim(frac, "0123456789") != "" || len(intPart) > 15 {
		return 0, errBadAmount
	}
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, errBadAmount
	}
	cents, _ := strconv.ParseInt((frac + "00")[:2], 10, 64)
	total := whole*100 + cents
	if neg {
		total = -total
	}
	return total, nil
}

// ungroup removes thousands separators, requiring well-formed groups: 1 to 3 digits, then groups of exactly 3.
func ungroup(s string, sep byte) (string, bool) {
	if !strings.ContainsRune(s, rune(sep)) {
		return s, true
	}
	groups := strings.Split(s, string(sep))
	if len(groups[0]) == 0 || len(groups[0]) > 3 {
		return "", false
	}
	for _, g := range groups[1:] {
		if len(g) != 3 {
			return "", false
		}
	}
	return strings.Join(groups, ""), true
}
