# Registration lock — plan

**Date:** 2026-10-11
**Status:** to implement

## Problem

`gastos.gupa.dev` is public and `POST /api/auth/register` accepts anyone. Each new account creates a wallet and writes to the database. For a personal app this is only attack surface (abuse, junk, bcrypt cost on the VDI CPU).

## Decision

Registration **closed by default**, with two controlled exceptions:

| Situation | Registration |
|---|---|
| Database with no users | open, to create the first account |
| `GASTOS_INVITE_CODE` set | requires the code at sign-up (to invite Bia, for example) |
| Database with users and no code | **closed** (`403`) |

Why a code and not an email invite: the app has no way to send email, and the code is a secret you pass to the person however you prefer. To end invites, just remove the variable and restart.

## Backend

- `config.Config` gets `InviteCode string` (env `GASTOS_INVITE_CODE`, empty by default). Minimum of 8 characters when set; `Load` fails early if it is shorter.
- `GET /api/auth/registration` (public) returns `{"mode":"open"|"code"|"closed"}`: `open` if there are no users, `code` if a code is configured, `closed` otherwise. The frontend uses it to build the screen.
- `POST /api/auth/register`:
  - `closed` → `403` with `code: "forbidden"` and the message "O cadastro está fechado. Peça um convite." (Registration is closed. Ask for an invite.).
  - `code` → `inviteCode` field required; constant-time comparison (`subtle.ConstantTimeCompare`); wrong code → `403` with the same generic message (does not reveal whether the sign-up exists).
  - `open` → as today. The first account is still created in a transaction, and the mode is recomputed inside the transaction so two simultaneous requests do not create two "first" accounts.
  - Goes through the same rate limit as login (10 per 15 minutes, per IP).
- `register` ignores `inviteCode` when the mode is `open`.

## Frontend

- `Login` queries `/api/auth/registration` on mount.
  - `closed`: hides "Criar uma conta" (Create an account).
  - `code`: shows the "Código de convite" (Invite code) field in the sign-up form.
  - `open`: as today (only happens for the first account).
- A `403` error on sign-up appears as a general message.

## Tasks

- [ ] **1. Config** (test first): empty and valid `GASTOS_INVITE_CODE` pass; fewer than 8 characters fails.
- [ ] **2. Registration mode** (test first): pure function `registrationMode(userCount, code)`; table tests for the three situations.
- [ ] **3. `/api/auth/registration` endpoint** and the lock in `register`, with integration tests:
  - first account passes without a code;
  - second account without a code → `403`;
  - with the right code → `201`; with a wrong or missing code → `403`;
  - two simultaneous accounts with no users: only one is created (`t.Parallel` + goroutines);
  - the rate limit also applies to sign-up.
- [ ] **4. Frontend**: `Login` per mode (component tests for the three modes); e2e: sign-up with a code and closed without one.
- [ ] **5. Docs**: `docs/ops/vdi-setup.md` (variable in `production.env`, how to invite and how to close) and README.

## Out of scope

Email invites, single-use invite links, manual account approval.

## Risk

Anyone who already has an account is not affected. Anyone who forgets to set the code after creating the first account ends up with registration closed, which is the safe behavior.
