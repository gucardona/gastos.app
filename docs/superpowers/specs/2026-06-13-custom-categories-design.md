# Custom Categories & Payment Methods

**Date:** 2026-06-13
**Status:** Approved

## Problem

Categories and payment methods are hardcoded JS arrays in the frontend. Users cannot add, rename, recolor, or remove them. All accounts share the same fixed set regardless of their needs.

## Goal

Allow each account to fully customize its expense categories and payment methods: add new ones, edit name/emoji/color/essentiality, delete with reassignment of existing expenses.

## Decisions

- **Scope:** per-account (not per-user)
- **Defaults:** seeded into the DB at account creation; treated identically to custom entries thereafter
- **On delete:** user must pick a replacement; expenses are reassigned in the same transaction
- **Emoji input:** curated popup grid (~150 emojis in sections), no external library
- **Color input:** native `<input type="color">`

---

## Data Model

### `account_categories`

```sql
CREATE TABLE IF NOT EXISTS account_categories (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id   INTEGER NOT NULL,
    key          TEXT    NOT NULL,
    name         TEXT    NOT NULL,
    icon         TEXT    NOT NULL,
    color        TEXT    NOT NULL,
    essentiality TEXT    NOT NULL CHECK(essentiality IN ('essential','nonessential','investment')),
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, key),
    FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
);
```

- `key` is a stable URL-safe slug stored in `expenses.category`. Set server-side on creation from a slugified name (collision → append `_2`, `_3`, etc.). **Immutable after creation.**
- Editing a category changes `name`, `icon`, `color`, `essentiality` only — `key` never changes, so no expenses break.

### `account_payment_methods`

```sql
CREATE TABLE IF NOT EXISTS account_payment_methods (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    key        TEXT    NOT NULL,
    icon       TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, key),
    FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE
);
```

- Same `key` semantics as categories. Stored in `expenses.payment`.

### Seeding defaults

The 15 current hardcoded categories and 8 payment methods are inserted as rows when an account is created (`CreateAccountWithOwner` in `db.go`, same transaction).

A named migration (`account_customization_v1`) seeds defaults for all existing accounts that have no rows yet in these tables.

---

## API

All routes: JWT required + `X-Account-ID` header. GET is available to all roles. POST/PATCH/DELETE require `owner` or `editor`.

### Categories

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/categories` | List all for active account, ordered by `sort_order` |
| POST | `/api/categories` | Create. Body: `name`, `icon`, `color`, `essentiality` |
| PATCH | `/api/categories/:key` | Update `name`, `icon`, `color`, `essentiality`, `sort_order` |
| DELETE | `/api/categories/:key` | Delete. Body: `replacementKey`. In one transaction: UPDATE expenses + DELETE row |

### Payment Methods

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/payment-methods` | List all for active account, ordered by `sort_order` |
| POST | `/api/payment-methods` | Create. Body: `name`, `icon` |
| PATCH | `/api/payment-methods/:key` | Update `name`, `icon`, `sort_order` |
| DELETE | `/api/payment-methods/:key` | Delete. Body: `replacementKey`. In one transaction: UPDATE expenses + DELETE row |

### Validation

- `name`: required, non-empty after trim
- `icon`: required, non-empty
- `color`: must match `#[0-9a-fA-F]{6}`
- `essentiality`: must be `essential`, `nonessential`, or `investment`
- On DELETE: `replacementKey` must exist in the same account and differ from the key being deleted; account must have at least 2 entries so a replacement always exists

### Key generation (POST)

Server slugifies `name` → lowercase, strip non-alphanumeric, replace spaces with `_`. Check uniqueness within account. If collision, append `_2`, `_3`, etc.

---

## New Files

- `src/handlers/categories.go` — GET/POST/PATCH/DELETE for categories
- `src/handlers/payment_methods.go` — GET/POST/PATCH/DELETE for payment methods

## Modified Files

- `src/db/db.go` — new tables, seeding in `CreateAccountWithOwner`, `account_customization_v1` migration
- `src/models/models.go` — `Category` and `PaymentMethod` structs
- `src/main.go` — register 8 new routes
- `src/web/index.html` — remove hardcoded arrays, load from API, settings UI

---

## Frontend

### State

```js
categories: [],       // loaded from GET /api/categories — replaces hardcoded array
paymentMethods: [],   // loaded from GET /api/payment-methods — replaces hardcoded array
```

Loaded in `loadAllData()` alongside expenses, incomes, etc. The rest of the app (expense form selects, charts, filter chips, badges) already reads `this.categories` and `this.paymentMethods` — no changes needed there.

On account switch: existing `loadAllData()` call reloads both lists automatically.

### Settings page — two new cards

**"Categorias" card:**
- List rows: `[icon] Name · essentiality chip · color swatch | [Editar] [Remover]`
- "Adicionar categoria" button opens an inline form below the list
- Inline add/edit form fields: emoji button (opens popup), name input, essentiality select, color input (`<input type="color">`)
- One active form at a time (editing one collapses any other open form)

**"Métodos de pagamento" card:**
- List rows: `[icon] Name | [Editar] [Remover]`
- Inline add/edit form: emoji button, name input

**Delete flow (inline, no modal):**
- Clicking Remover on a row reveals an inline confirmation: "Mover gastos para:" + dropdown of other entries + "Confirmar" button
- On confirm: calls DELETE endpoint, on success removes from local list and reloads expenses

### Emoji popup

- Triggered by clicking the emoji button on the icon field
- Floating div (absolute positioned) with a grid of ~150 curated emojis
- Organized in labeled sections: Comida, Transporte, Saúde, Finanças, Casa, Lazer, Outros
- Clicking an emoji: sets the icon field, closes popup
- Clicking outside (Alpine `@click.away`): closes popup
- State: `emojiPickerOpen: false`, `emojiPickerTarget: null` (tracks which form field is active)

### Curated emoji list (hardcoded in JS, ~150 entries)

Sections and representative emojis:
- **Comida:** 🍽 🍜 🍕 🍔 🥗 🛒 🥩 🍺 ☕ 🧃 🍰 🍣
- **Transporte:** 🚗 🚕 🚌 🚆 ✈️ 🛵 🚲 ⛽ 🅿️ 🚁
- **Saúde:** 💊 🏥 🦷 👓 🧬 💉 🩺 🏃 🧘
- **Finanças:** 💰 💳 📈 📉 🏦 💵 💸 🔐 📊 🪙
- **Casa:** 🏠 🛋 💡 🔧 🧹 📦 🪴 🛁 🔑
- **Lazer:** 🎮 🎬 📚 🎵 ⚽ 🏋️ 🎭 🎲 🏖 🎨
- **Outros:** ✨ 📺 📱 👕 💄 🐾 🎁 📝 ◈ 🔔

---

## Models

```go
type Category struct {
    ID           int64  `json:"id"`
    AccountID    int64  `json:"-"`
    Key          string `json:"key"`
    Name         string `json:"name"`
    Icon         string `json:"icon"`
    Color        string `json:"color"`
    Essentiality string `json:"essentiality"`
    SortOrder    int    `json:"sortOrder"`
}

type PaymentMethod struct {
    ID        int64  `json:"id"`
    AccountID int64  `json:"-"`
    Key       string `json:"key"`
    Icon      string `json:"icon"`
    Name      string `json:"name"`
    SortOrder int    `json:"sortOrder"`
}
```

---

## Error Handling

- GET on an account with no rows: trigger seeding lazily (fallback if migration missed it), return defaults
- DELETE with `replacementKey` that doesn't exist: 400 Bad Request
- DELETE when only 1 entry remains: 400 "Mantenha ao menos uma categoria"
- PATCH on non-existent key: 404
- POST with duplicate name that generates a colliding key: server resolves silently by appending suffix

---

## Out of Scope

- Drag-to-reorder (sort_order can be patched manually later if needed)
- Per-user (not per-account) customization
- Category icons beyond a single emoji (no image upload)
- Syncing custom categories across accounts
