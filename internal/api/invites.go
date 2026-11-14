package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"math/big"
	"strings"
	"time"
)

// inviteAlphabet is Crockford base32 without 0 and 1 (no I, L, O, U either), so a code read aloud or
// copied by hand has no look-alike characters. 12 characters of 30 symbols give about 59 bits.
const inviteAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

// newInviteCode returns a random code formatted XXXX-XXXX-XXXX.
func newInviteCode() (string, error) {
	var b strings.Builder
	for i := 0; i < 12; i++ {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteAlphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(inviteAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// normalizeInvite makes "k3j9 xq2m-7prt" and "K3J9-XQ2M-7PRT" the same code.
func normalizeInvite(code string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, code))
}

func inviteHash(code string) []byte {
	h := sha256.Sum256([]byte(normalizeInvite(code)))
	return h[:]
}

// CreateInvite stores a new single-use invite code valid for ttl from now and returns it; only its hash is kept.
func CreateInvite(ctx context.Context, d *sql.DB, now time.Time, ttl time.Duration) (string, time.Time, error) {
	code, err := newInviteCode()
	if err != nil {
		return "", time.Time{}, err
	}
	exp := now.UTC().Add(ttl)
	if _, err := d.ExecContext(ctx, `INSERT INTO invite_codes (code_hash, expires_at) VALUES (?, ?)`,
		inviteHash(code), exp.Format(tsLayout)); err != nil {
		return "", time.Time{}, err
	}
	return code, exp, nil
}
