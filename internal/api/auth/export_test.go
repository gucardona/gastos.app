package auth

import "net/http"

// InviteAlphabet exposes inviteAlphabet to the tests.
const InviteAlphabet = inviteAlphabet

// NewInviteCode exposes newInviteCode to the tests.
var NewInviteCode = newInviteCode

// NormalizeInvite exposes normalizeInvite to the tests.
var NormalizeInvite = normalizeInvite

// RegistrationMode exposes registrationMode to the tests.
type RegistrationMode = registrationMode

// The registration modes, exposed to the tests.
const (
	ModeOpen   = modeOpen
	ModeCode   = modeCode
	ModeClosed = modeClosed
)

// ModeFor exposes modeFor to the tests.
var ModeFor = modeFor

// ClientIP exposes clientIP to the tests.
func ClientIP(r *http.Request) string { return clientIP(r) }
