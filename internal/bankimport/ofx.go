package bankimport

import (
	"errors"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotOFX         = errors.New("this does not look like an OFX file")
	ErrNoTransactions = errors.New("no transactions found in the file")
	ErrTooLarge       = errors.New("file is too large")
)

const stmtTrn = "<STMTTRN>"

// ParseOFX reads an OFX statement, either 1.x (SGML: tags are never closed) or 2.x (XML), of a bank account or a card.
// Lines with a zero amount (fee-free notices, balances) are skipped. Errors name the transaction by position.
func ParseOFX(r io.Reader) ([]Entry, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	text := decodeText(raw)
	upper := strings.ToUpper(text)
	if !strings.Contains(upper, "<OFX") && !strings.Contains(upper, "OFXHEADER") {
		return nil, ErrNotOFX
	}

	// Without closing tags the only reliable boundary is the next <STMTTRN>, so slice between starts.
	var starts []int
	for from := 0; ; {
		i := strings.Index(upper[from:], stmtTrn)
		if i < 0 {
			break
		}
		starts = append(starts, from+i)
		from += i + len(stmtTrn)
	}

	var (
		entries []Entry
		seenID  = map[string]int{}
		seenRow = map[string]int{}
	)
	for n, st := range starts {
		end := len(text)
		if n+1 < len(starts) {
			end = starts[n+1]
		}
		block := text[st+len(stmtTrn) : end]
		if c := strings.Index(strings.ToUpper(block), "</STMTTRN>"); c >= 0 {
			block = block[:c]
		}

		date, err := ofxDate(tagValue(block, "DTPOSTED"))
		if err != nil {
			return nil, fmt.Errorf("transaction #%d: %w", n+1, err)
		}
		cents, err := ParseAmount(tagValue(block, "TRNAMT"))
		if err != nil {
			return nil, fmt.Errorf("transaction #%d: amount %q: %w", n+1, tagValue(block, "TRNAMT"), err)
		}
		if cents == 0 {
			continue
		}
		desc := tidy(html.UnescapeString(tagValue(block, "MEMO")))
		if desc == "" {
			desc = tidy(html.UnescapeString(tagValue(block, "NAME")))
		}
		if desc == "" {
			desc = NoDescription
		}

		id := tidy(tagValue(block, "FITID"))
		if id == "" {
			key := fmt.Sprintf("%s|%d|%s", date, cents, desc)
			id = fallbackID(date, cents, desc, seenRow[key])
			seenRow[key]++
		}
		entries = append(entries, Entry{ExternalID: uniqueID(seenID, id), Date: date, AmountCents: cents, Description: desc})
	}
	if len(entries) == 0 {
		return nil, ErrNoTransactions
	}
	return entries, nil
}

var tagRes = map[string]*regexp.Regexp{}

// tagValue returns the text after the first <TAG> in block, up to the next tag or line break
// (which covers both SGML, `<MEMO>text`, and XML, `<MEMO>text</MEMO>`).
func tagValue(block, tag string) string {
	re, ok := tagRes[tag]
	if !ok {
		re = regexp.MustCompile(`(?i)<` + tag + `>\s*([^<\r\n]*)`)
		tagRes[tag] = re
	}
	if m := re.FindStringSubmatch(block); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// ofxDate keeps the first eight digits (YYYYMMDD); anything after them is a time and a zone offset that we ignore.
func ofxDate(v string) (string, error) {
	if len(v) < 8 {
		return "", fmt.Errorf("invalid date %q", v)
	}
	d, err := time.Parse("20060102", v[:8])
	if err != nil {
		return "", fmt.Errorf("invalid date %q", v)
	}
	return d.Format("2006-01-02"), nil
}
