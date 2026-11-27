# Interface restructuring, PR 3: Lançamentos and Caixa

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the new language to Lançamentos (Transactions) and to Caixa (Inbox), and move Recorrentes (Recurrences) into Lançamentos as the "Repetem todo mês" (Repeat every month) section, without losing any current behavior.

**Architecture:** Lançamentos gets the segmented filter, the search and the totals in the Visão (Overview) style, and a "Repetem todo mês" section at the end of the page (grouped list, switch to pause, tapping the row opens an edit sheet). The Caixa goes from a form per row to a grouped list: tapping a row opens a sheet with the category (grid), the description, the optional rule and the actions. Both sheets reuse `Sheet`, `ListGroup`/`ListRow`, `Switch`, `Segmented` and `pickers` from PR 2. The `/recorrentes` page stops existing as a destination and redirects to Lançamentos.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop and Pixel 5), plain CSS with the PR 1 tokens.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), sections Screens (Lançamentos, Caixa), Navigation and Delivery (PR 3). Base: PRs 1 and 2 on master.

## Global Constraints

- Code, comments and tests in English; interface text in pt-BR.
- Same system as PR 2: grouped blocks with radius 22, controls with radius 12, no shadows, text on the left and values on the right, touch targets of at least 44 px, visible focus, AA contrast in both themes, and only tokens from `tokens.css` (no new color).
- Sheets are `Sheet` (native dialog); the category choice is a `PickerView` inside the sheet itself, as in the transaction sheet.
- Expenses are not red; income in `--income`; `--over` only for alerts and destructive actions (Ignorar (Ignore) and Excluir (Delete) are secondary, deletion asks for confirmation).
- Recorrentes leaves the navigation and the route: `/recorrentes` now redirects to `/lancamentos`. The "Repetem todo mês" section shows up for those who can edit and for readers (readers see it without controls).
- The API, the data and the rules do not change. Every behavior covered today by `Transactions.test.tsx`, `Recurring.test.tsx` and `Inbox.test.tsx` stays covered (same behaviors, updated selectors).

## Review Focus

- Pausing a recurrence sends the whole object with `enabled=false` (and resuming, with `true`); settlement recurrences (with no category) show up and edit without breaking. Task 3.
- Reader: no switch and no tapping on recurrence rows, and no actions in the Caixa. Task 3 and 4.
- Whoever opens `/recorrentes` from a bookmark lands on Lançamentos, not on a blank page. Task 3.
- Caixa: a row that someone else just handled (409) shows the server message and the list refreshes; an income row only offers income categories. Task 4.
- Category rule: text shorter than 2 characters is refused before accepting the row (today the row would be accepted and the rule would fail). Task 4.
- Caixa with many rows (200 or more): the list scrolls, "Aceitar N sugeridas" (Accept N suggested) counts only the safe ones. Task 4.
- Lançamentos: search and filter combined, totals for the whole month, and a month without transactions with the recurrences still visible. Task 1 and 3.
- No horizontal scroll between 900 and 1100 px in Lançamentos and in the Caixa. Task 5.

---

## File structure

| File | Responsibility |
|---|---|
| `web/src/pages/Transactions.tsx` (modifies) | Segmented filter, search and totals in the new style; includes `RecurringSection` |
| `web/src/components/RecurrenceSheet.tsx` (new) | Sheet for editing and deleting a recurrence; exports `recurrenceBody` |
| `web/src/components/RecurringSection.tsx` (new) | "Repetem todo mês" section |
| `web/src/pages/Recurring.tsx`, `Recurring.test.tsx` (deletes) | Replaced by the two files above |
| `web/src/App.tsx` (modifies) | `/recorrentes` redirects to `/lancamentos` |
| `web/src/components/InboxSheet.tsx` (new) | Decision sheet for one bank statement row |
| `web/src/pages/Inbox.tsx` (modifies) | Grouped list that opens `InboxSheet` |
| `web/src/styles.css` (modifies) | New styles; removes `.filters`, `.search` and `.item`/`.inline-form` that end up unused (check with `grep`; Ajustes (Settings) still uses `.item` and `.inline-form` until PR 4) |

---

### Task 1: Lançamentos in the new style

**Files:**
- Modify: `web/src/pages/Transactions.tsx`, `web/src/pages/Transactions.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Segmented` (PR 2).
- Produces: the page keeps the accessible name of the controls: search `Buscar` (`type="search"`), group `Filtros` with the buttons `Tudo`, `Gastos`, `Entradas` (`aria-pressed`).

- [ ] **Step 1: Tests first** in `Transactions.test.tsx`: keep all the current behaviors (month totals and list by default, case-insensitive search, expenses filter and income filter, filter with search with whole-month totals, empty month invites creating, reader without the invitation, right month in the API, grouped by day, error with retry, loading). Add: the filter is a `Filtros` group with three buttons and only `Tudo` selected at the start; the totals show "Entrou" (Came in) and "Saiu" (Went out) with the same dot markup as Visão (`.flows`); the search field has a minimum height of 44 px (`search` class with the rule in the CSS, verified in Step 4).
- [ ] **Step 2: Run and see the new tests fail.**
- [ ] **Step 3: Implement.** Replace the chips with `<Segmented label="Filtros" ...>` with the three values; the totals now use the `.flows` block from Visão; the search gets the rounded field style with radius 12 and a surface background. Remove the temporary "Repetem todo mês ›" link (the section comes in Task 3).
- [ ] **Step 4: CSS:** `.search` with `min-height: 44px`; delete `.filters` and `.chip` only if `grep` shows that no other component uses them (Ajustes and Caixa still use `.chip`).
- [ ] **Step 5: Run the whole suite.** Expected: all green.
- [ ] **Step 6: Commit** `feat(web): Lançamentos with the segmented filter and the new totals`.

---

### Task 2: Recurrence sheet

**Files:**
- Create: `web/src/components/RecurrenceSheet.tsx`, `web/src/components/RecurrenceSheet.test.tsx`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `useAction` (existing hook in `hooks/useAction.ts`).
- Produces:
  - `function recurrenceBody(r: Recurrence, over: Partial<Recurrence>, splitting: boolean): object`: the complete PUT body (`kind, amountCents, description, categoryId, paymentMethodId, paidByUserId, toUserId, dayOfMonth, startDate, endDate, enabled`), with `splits` (only `userId` and `shareBp`) only when `splitting` and the kind is expense and there are parts. This is the logic that lives inside `Recurring.tsx` today.
  - `function RecurrenceSheet(props: { recurrence: Recurrence | null; onClose: () => void }): JSX.Element`: `Sheet` with the label "Editar recorrência" (Edit recurrence); closed when `recurrence` is `null`.

Sheet content: amount (`Valor` field), description (`Descrição`), day of the month (`Dia do mês`), optional end date (`Data final`), "Salvar" (Save) button, and "Excluir recorrência" with confirmation ("Confirmar exclusão"). Validation messages as today: "Informe um valor maior que zero." and "O dia deve estar entre 1 e 31."

- [ ] **Step 1: Tests in `RecurrenceSheet.test.tsx`**, porting the behaviors covered today in `Recurring.test.tsx`: edits amount and day validating before sending (zero amount and day 0 or 32 do not send and show the message), sends the whole object by PUT to `/api/wallets/1/recurrences/{id}` preserving the fields the sheet does not show; only sends the split parts when the wallet splits expenses and the kind is expense; deletes only after confirming (DELETE); closes on save and on delete; a settlement recurrence (`kind: "settlement"`, `categoryId: null`) opens and saves without requiring a category. Unit tests for `recurrenceBody`: with `over` applied, without `splits` when the wallet does not split or the kind is income or settlement, with `splits` mapped when it splits.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** `recurrenceBody` and `RecurrenceSheet` (form inside the sheet, local state, `useAction` for the call and the error).
- [ ] **Step 4: Run and see them pass.**
- [ ] **Step 5: Commit** `feat(web): recurrence edit sheet`.

---

### Task 3: "Repetem todo mês" section and the end of the Recorrentes page

**Files:**
- Create: `web/src/components/RecurringSection.tsx`, `web/src/components/RecurringSection.test.tsx`
- Modify: `web/src/pages/Transactions.tsx`, `web/src/pages/Transactions.test.tsx`, `web/src/App.tsx`, `web/src/styles.css`
- Delete: `web/src/pages/Recurring.tsx`, `web/src/pages/Recurring.test.tsx`

**Interfaces:**
- Consumes: `recurrenceBody`, `RecurrenceSheet` (Task 2), `ListGroup`, `ListRow`, `Switch`, `useRecurrences`, `useAction`.
- Produces: `function RecurringSection(): JSX.Element`: title `Repetem todo mês` (`<h2>`), a `ListGroup` with one row per recurrence (category icon or ⇄ for settlement, title, "Todo dia N, próxima em D de mês" (Every day N, next on D of month) / ", pausada" (paused) / ", encerrada" (ended), signed amount, and a `Switch` named `Pausar <título>` (Pause) or `Retomar <título>` (Resume)); tapping the row opens `RecurrenceSheet`; with no recurrences it shows the hint "Nenhuma recorrência ainda. Ao lançar um gasto ou uma entrada, ative Repetir todo mês." (for those who can edit) or nothing (reader).

- [ ] **Step 1: Tests in `RecurringSection.test.tsx`**, porting `Recurring.test.tsx`: list with day, next date and amount; empty state with the hint; pausing sends the whole object with `enabled=false` (and resuming with `true`); only sends split parts when the wallet splits; reader sees the list without a switch and without opening anything; tapping the row opens the edit sheet of the right recurrence; settlement recurrence shows "Acerto de Ana para Bia" (Settlement from Ana to Bia). In `Transactions.test.tsx`: the section shows up on the page (with the transaction list above it), including when the month has no transactions; replace the test for the "Repetem todo mês" link with one for the section. Route test in `App.test.tsx` (new, with `MemoryRouter` at `/recorrentes`): ends up in Lançamentos.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** `RecurringSection` (rows as `ListRow` without `onClick`, nesting an open button and the `Switch`: the row has two controls, so the clickable content is a `list-row` button on the left and the `Switch` sits beside it, outside the button), include it at the end of `Transactions`, replace the route with `<Route path="recorrentes" element={<Navigate to="/lancamentos" replace />} />`, delete `Recurring.tsx` and `Recurring.test.tsx`, and remove the styles that end up unused.
- [ ] **Step 4: Run the whole suite** (`npx vitest run`, `tsc`, `npm run lint`). Expected: green.
- [ ] **Step 5: Commit** `feat(web): recurrences live inside Lançamentos`.

---

### Task 4: Caixa

**Files:**
- Create: `web/src/components/InboxSheet.tsx`, `web/src/components/InboxSheet.test.tsx`
- Modify: `web/src/pages/Inbox.tsx`, `web/src/pages/Inbox.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `Switch`, `PickerView`, `CategoryGrid`, `useAction`, `InboxItem` (types).
- Produces: `function InboxSheet(props: { item: InboxItem | null; onClose: () => void }): JSX.Element`. Content: amount and date (read-only), **Categoria** row (opens the grid with only categories of the row's type, the suggested one already selected), **Descrição** field (starts with the suggested name or the bank statement text), when there is no suggested category a **Usar nas próximas importações** (Use on the next imports) row with a `Switch` and, when on, the **Texto da regra** (Rule text) field (starts with the first two words of the description in lowercase); actions **Aceitar** (Accept) (disabled without a category), **É este lançamento** (This is that transaction) (only with a candidate) and **Ignorar**.

The `Inbox` page shows "Aceitar N sugeridas" at the top when there are any, and the rows in a `ListGroup` (description or suggested name, "date, account" and signed amount; a second small line "Parece o lançamento que você já fez: …" (Looks like the transaction you already made: …) when there is a candidate); tapping opens `InboxSheet`; a reader sees the rows without opening anything. Empty: the same explanation as today with the link to `/ajustes?s=bancos`.

- [ ] **Step 1: Tests** porting the 11 behaviors from `Inbox.test.tsx` (empty points to settings; accepts with the chosen category and sends only `{ categoryId }` plus the description when edited; does not accept without a category; only offers categories of the row's type; starts from the suggested category and name; creates the rule after accepting; links to the similar transaction instead of creating; ignores; accepts all suggested ones without counting the ones that need a decision; shows the server error; reader only looks), now through the sheet: tap the row, choose in the grid, tap Aceitar. Add: a rule with text shorter than 2 characters is refused **before** calling the API ("Use ao menos 2 caracteres na regra."); a row handled by someone else (409) shows "Esta linha já foi tratada" and the sheet stays open; edited description is sent; with 250 rows the list renders all of them and the button says "Aceitar N sugeridas" with N counting only the safe ones.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** `InboxSheet` (same pattern as `AddSheet`: form state in the component, `picker` for the grid, focus returned to the row) and rewrite `Inbox`. The accept call remains `POST /inbox/{id}/accept` followed, if requested, by `POST /rules`.
- [ ] **Step 4: Run the whole suite.** Expected: green.
- [ ] **Step 5: Commit** `feat(web): Caixa as a grouped list with a decision sheet`.

---

### Task 5: e2e, screenshots and wrap-up

**Files:**
- Modify: `web/e2e/app.spec.ts`

- [ ] **Step 1: Adjust the e2e tests:** the "settings and recurring" flow reaches the recurrences through Lançamentos (the "Repetem todo mês" section), pauses with the `Pausar Netflix` switch and checks "pausada"; the OFX flow decides the Caixa rows through the sheet (tap the row, choose the category in the grid, "Aceitar"; and "Ignorar").
- [ ] **Step 2: New e2e tests:** opening `/recorrentes` leads to Lançamentos; editing a recurrence's amount through the sheet and seeing the new amount in the section; and, on desktop at 900 px, Lançamentos and Caixa have no horizontal scroll (same check as PR 2, with long data).
- [ ] **Step 3: Run** `npm run build && npm run e2e`. Expected: all pass, in both projects.
- [ ] **Step 4: Screenshots** of Lançamentos (with the recurrences section) and of the Caixa, mobile and desktop, light and dark, with a temporary spec that is not committed; fix whatever departs from the approved design.
- [ ] **Step 5: Open the PR** describing what changed and that the merge triggers the deploy (the `master` branch requires `--admin`).
- [ ] **Step 6: Commit** `test(e2e): recurrences inside Lançamentos and the Caixa sheet`.
