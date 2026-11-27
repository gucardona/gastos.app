# Frontend — React rewrite

**Date:** 2026-10-09
**Status:** Draft — the visual direction needs mockup approval before implementation
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

## Goal

Replace `src/web/index.html` (165 KB, Alpine + Chart.js + flatpickr via CDN) with a typed, minimalist and robust React app that consumes the v2 API from the [backend spec](2026-10-09-backend-foundation-design.md).

Robust here means:
- end-to-end types;
- loading, error and empty states on every screen;
- no lost form data;
- works well on a phone.

Minimalist means:
- fewer screens and fewer clicks for the common case, which is logging an expense;
- typographic hierarchy instead of colored cards;
- a single accent color.

## Stack

| Layer | Choice | Reason |
|---|---|---|
| Build | Vite + TypeScript (strict) | fast, standard |
| UI | React 19 | |
| Style | Tailwind CSS v4 + shadcn/ui-style components (Radix underneath) | accessibility out of the box (dialog, popover, select), component code lives in the repo |
| Routing | TanStack Router | typed search params: `?m=2026-10&cat=3` becomes validated state in the URL |
| Data | TanStack Query | cache, SSE invalidation, optimistic updates |
| Forms | react-hook-form + zod | same validation as the backend, field errors from the API are mapped |
| Charts | Recharts | few charts, all simple |
| Dates | date-fns (`pt-BR` locale) | |
| Icons | lucide-react | categories still accept an emoji as the icon |
| Tests | Vitest + Testing Library; Playwright for e2e smoke | |

No global state manager: server state lives in TanStack Query, URL state in the router and the rest in `useState`.

## Structure

```
web/
  index.html
  embed.go                 # package web; //go:embed all:dist
  src/
    main.tsx, router.tsx
    api/                   # fetch client (credentials:'include'), types, query hooks per resource
    lib/money.ts           # cents ↔ pt-BR string, input parsing "12,90"
    lib/dates.ts
    components/ui/         # primitives (button, input, dialog, sheet, select, combobox, toast...)
    components/            # MoneyInput, CategoryPicker, MonthSwitcher, TransactionRow, EmptyState...
    features/
      auth/  overview/  transactions/  recurring/  budgets/  split/  settings/  inbox/ (phase 4)
    styles.css             # tokens (CSS vars) + tailwind
```

## Information architecture

There used to be 7 tabs: dashboard, expenses, income, payments, goals, charts and settings. Now there are **5 destinations**:

| Route | Content | Replaces |
|---|---|---|
| `/` **Visão geral** (Overview) | month summary: balance, income, expenses, daily average, comparison with the previous month; expenses by category with a budget bar; latest transactions; "acertos" (settlements) if splitting is enabled | dashboard + charts + goals (viewing) |
| `/lancamentos` | single list of the month's expenses and income, grouped by day; filters (type, category, payment method, search); totals at the top | expenses + income |
| `/recorrentes` | recurring templates (expense, income, settlement) with an on/off toggle and next date | the three "recurring" blocks scattered around |
| `/divisao` | balances between members, suggested settlements (with a "registrar acerto" button), settlement history; only shown with `splittingEnabled` | payments |
| `/ajustes` | wallet (name, splitting), members, categories, payment methods, budgets, account (logout), theme | settings + goals (editing) |

Phase 4 adds **`/caixa`** (inbox of imported transactions), with a pending-items badge.

The selected month lives in the URL (`?m=2026-10`) and is shared between Visão geral and Lançamentos (Transactions). The wallet selector sits at the top of the layout. The active wallet is stored in `localStorage`, with a fallback to the first wallet from `/api/me`.

## Quick transaction entry

This is the most important flow in the app.

- The "+" button (FAB on mobile, header button on desktop) and the `N` shortcut open a **sheet** (from the bottom on mobile, from the side on desktop).
- The field order is designed for the thumb:
  - **Gasto / Entrada** (Expense / Income) toggle;
  - **amount** with a numeric keyboard (`inputmode="decimal"`), already focused;
  - **description**;
  - **category** as a grid of chips with the most used first;
  - payment method;
  - date (default today, with "ontem" (yesterday) and "hoje" (today) shortcuts);
  - "quem pagou" (who paid) and split, only if splitting is enabled, collapsed into "Dividir: igual entre 2 ▾" (Split: equal between 2).
- The "Repetir todo mês" (Repeat every month) toggle turns the transaction into a recurrence without changing screens.
- `Enter` saves; "Salvar e lançar outro" (Save and add another) keeps the sheet open with type, category and date preserved.
- The update is optimistic: the item shows up in the list right away and is reverted with an error toast if the API fails.
- The draft is kept in `sessionStorage` while the sheet is open. Closing it by accident does not lose what was typed.

Editing uses the same sheet. Deleting shows an "Desfazer" (Undo) action in the toast (5 seconds) before calling the API.

## Real time

`EventSource('/api/wallets/{id}/events')` with the cookie, no token. On every `changed` event, it calls `queryClient.invalidateQueries({ queryKey: ['wallet', id] })`. The hook reconnects on its own and closes when the wallet changes.

## Visual direction (proposal, to be approved)

- **Tone:** calm and editorial, like a well laid out bank statement, not a bank dashboard.
- **Neutral base:** off-white in light (`#FAFAF9`), graphite in dark (`#0C0C0D`).
- **One accent color:** teal green, inheriting the current `#0f6579`, more saturated in dark.
- **Semantics:** expenses in neutral and income in green. Red only for budget overruns and errors, so the screen is not painted red in a normal month.
- **Typography:** Inter (UI) with `font-variant-numeric: tabular-nums` on every amount. Large summary amounts at weight 600, size 32–40px. No decorative font.
- **Density:** lists with 52–56px rows:
  - category icon;
  - description and subtitle (category · payment method);
  - amount aligned to the right.
  No cards inside cards.
- **Charts:**
  - horizontal bars of category × budget, replacing the pie chart;
  - one line of cumulative spending in the month against the previous month.
  Only these two.
- **Layout:**
  - mobile-first;
  - bottom nav with 4 items + FAB (Ajustes (Settings) lives in the avatar menu);
  - on desktop, a narrow sidebar and content with a 960px max width.
- **Theme:** light, dark or system, in CSS vars with `data-theme`.
- **Motion:** 150ms transitions on sheet, dialog and toast; respects `prefers-reduced-motion`.

**Before implementing:** produce a static mockup (HTML) of Visão geral, Lançamentos and the quick transaction sheet, in light and dark and at 375px and 1280px, for approval.

## API contract

- `api/client.ts`:
  - `fetch` with `credentials: 'include'`;
  - a `401` on any call redirects to `/login?next=...`;
  - errors become `ApiError {code, message, fields}`.
- Types live in `api/types.ts`, handwritten to mirror the backend spec. If the volume grows, migrate to generated OpenAPI.
- Money is always a `number` in cents. Only `MoneyInput` and `formatMoney` deal with reais.

## Accessibility and quality

- Full keyboard navigation. Visible focus. AA contrast in both themes.
- Every dialog and sheet has a focus trap (Radix).
- `EmptyState` on every list: first wallet, month with no transactions, no recurrences.
- Error boundary per route, with "tentar novamente" (try again).
- Minimal PWA: manifest and icons, for "add to home screen" on the phone, reusing the current icons. No offline service worker for now.

## Tests

- **Vitest:**
  - `lib/money` (parse and format, including "1.234,56", "12,9" and "0,05");
  - field error mapping;
  - split logic in the form.
- **Testing Library:** quick transaction sheet (validation, save and add another, draft).
- **Playwright (smoke against the real binary with a temporary DB):**
  - sign up;
  - log an expense and an income;
  - see the summary;
  - create a recurrence;
  - invite a member and split an expense.

## Acceptance criteria

- Functional parity with the current `index.html`. Checklist derived from the features:
  - auth;
  - wallets and members with roles;
  - customizable categories and payment methods;
  - expenses and income;
  - recurrences of the 3 types;
  - splitting and settlements;
  - budgets;
  - charts;
  - real time.
- Lighthouse mobile ≥ 90 in Performance and Accessibility on the Overview.
- Initial bundle ≤ 250 KB gzip. Recharts loaded on demand on the Overview.
- The old `src/web/` is removed in the same PR that turns on the `web/dist` embed.
