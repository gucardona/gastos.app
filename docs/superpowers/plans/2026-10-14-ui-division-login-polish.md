# Interface restructuring, PR 5: Divisão, login and finishing touches

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the restructuring: Divisão (Split) and login in the new language, the end of the old system's CSS and classes, and an automatic rule that keeps text from going back to being centered screen by screen.

**Architecture:** Divisão becomes a large sentence ("Bia deve R$ 120,00 a você" (Bia owes you R$ 120,00)) with the "Registrar acerto" (Record settlement) action, each person's balance in a grouped list and the history in a grouped list; creating and editing a settlement happens in a sheet (`SettlementSheet`), following the same pattern as the sheets in PRs 2 to 4. Login swaps the loose fields for field rows in the style of the sheets. The final pass deletes what is left unused (`.item`, `.line`, `.inline-form`, `.text`, `.field`, `.chip` and the `--display` variable) and a test reads the CSS and fails if a `text-align: center` shows up outside the spec's list of exceptions.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop and Pixel 5), plain CSS with the PR 1 tokens.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), sections Divisão, Login and sign-up, Alignment and Delivery (PR 5). Base: PRs 1 to 4 (PR 4 is open: this branch starts from it).

## Global Constraints

- Code, comments and tests in English; interface text in pt-BR.
- Same system as the previous PRs: grouped blocks with radius 22, controls with radius 12, no shadows, text on the left and values on the right, touch targets of at least 44 px, visible focus (including on fields that hide the browser outline), AA contrast in both themes, only tokens from `tokens.css`.
- **Alignment (system rule):** centered only in empty states (`.empty`), in the icon preview (`.preview`), in the transaction sheet amount (`.amount`), in category tiles (`.cat-tile`), in the choice header (`.picker-head`) and in the emoji grid (`.emoji-grid`). Any other `text-align: center` is an error.
- Deletion confirmation: the confirm button is a different element (different `key`), so a double click does not confirm.
- Divisão: only exists for a wallet that splits expenses (as today); the text saying who owes whom uses "Você" (You) for the user themselves and the first name of the others.
- Reader: sees everything and does not record, edit or delete anything.
- The API, the data and the rules do not change. Every behavior covered today by `Division.test.tsx` and `Login.test.tsx` stays covered.

## Review Focus

- Suggested settlement with 3 or more people: each suggested transfer shows up with its own button and the right name; "Vocês estão quites" (You are even) when there is no suggestion. Task 1.
- Manual settlement: invalid or zero amount and payer equal to receiver are refused before sending; editing keeps the date and the note; deleting only after confirming, and a double click does not confirm. Task 1.
- Reader and wallet without splitting: no write action; the owner sees "Ligar a divisão" (Turn on splitting) leading to `/ajustes/carteira`. Task 1.
- Login: wrong password, closed sign-up, invite code, different passwords and field errors keep the same message; Enter submits; the `autocomplete` and `type` attributes of the fields stay (password managers). Task 2.
- Ajustes (Settings) on a wide screen without a chosen section shows Perfil (Profile) in the right panel (not an empty panel), and there is only one visible `h1` per screen. Task 3.
- No `text-align: center` outside the alignment rule's list, and no old class remains in the CSS. Task 3.
- No horizontal scroll between 900 and 1100 px on Divisão, with long names. Task 4.

---

## File structure

| File | Responsibility |
|---|---|
| `web/src/pages/Division.tsx` (rewrites) | Sentence of who owes whom, balances, history |
| `web/src/components/SettlementSheet.tsx` (new) | Sheet to create, edit and delete a settlement |
| `web/src/pages/Login.tsx` (modifies) | Field rows in the new style |
| `web/src/pages/Settings.tsx` (modifies) | Profile as the default panel on wide screens; only one visible `h1` |
| `web/src/styles.css` (modifies) | Deletes the old system's CSS and the `--display` variable |
| `web/src/styles/tokens.css` (modifies) | Removes `--display` |
| `web/src/styles/alignment.test.ts` (new) | Guard for the alignment rule and the old classes |

---

### Task 1: Divisão

**Files:**
- Create: `web/src/components/SettlementSheet.tsx`, `web/src/components/SettlementSheet.test.tsx`
- Modify: `web/src/pages/Division.tsx`, `web/src/pages/Division.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `PickerView`, `OptionList`, `useAction`.
- Produces: `function SettlementSheet(props: { settlement: Settlement | "new" | null; onClose: () => void }): JSX.Element`. Rows **Quem pagou** (Who paid) and **Quem recebeu** (Who received) (each opens an `OptionList` with the members), fields `Valor do acerto`, `Data do acerto` (native date) and `Observação do acerto`, a Salvar (Save) button (or Registrar (Record), when creating) and, when editing, "Excluir acerto" (Delete settlement) with confirmation. Messages: "Informe um valor maior que zero." and "Escolha pessoas diferentes para quem paga e quem recebe."

The screen: title "Divisão"; for each suggested transfer, a large sentence (`<p class="big">`: "Bia deve R$ 120,00 a você") and the "Registrar acerto" button (only for those who can edit); with no suggestion, "Vocês estão quites."; "Registrar outro acerto" (Record another settlement) opens the sheet; each person's balance in a `ListGroup` (name, "tem a receber" (is owed), "deve" (owes) or "está quite" (is even), amount); the history in a `ListGroup` (who paid whom, date and note, amount), tapping a row opens the edit sheet.

- [ ] **Step 1: Tests** porting the 11 behaviors of `Division.test.tsx` (explains and offers to turn on splitting when it is off, without the button for those who do not administer; each person's balance and what is left to settle; "quites" with no suggestion; records the suggested settlement with today's date; records another settlement by hand validating amount and people; history with date, amount and note; edits a settlement from the history by PUT; deletes only after confirming; reader only looks; load error with retry) through the new components, and `SettlementSheet.test.tsx`: creating sends POST with `fromUserId`, `toUserId`, `amountCents`, `date`, trimmed `note`; editing comes pre-filled and sends PUT preserving date and note; zero or text amount and payer equal to receiver are refused without calling the API; deleting requires confirmation and a double click does not confirm; API error shows up without closing the sheet. Add: three people with two suggested transfers show two "Registrar acerto" buttons and each one sends its own.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the sheet and the screen (the logic of `useAction`, `api` and the messages stays; the inline form goes away).
- [ ] **Step 4: Run the whole suite.** Expected: green.
- [ ] **Step 5: Commit** `feat(web): Divisão with a sentence, grouped lists and a settlement sheet`.

---

### Task 2: Login and sign-up

**Files:**
- Modify: `web/src/pages/Login.tsx`, `web/src/pages/Login.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: nothing new; the fields keep `id`, `type`, `autoComplete`, the labels and today's messages.

- [ ] **Step 1: Tests** in `Login.test.tsx`: keep all the behaviors (signs in and goes to Visão (Overview), wrong password, open sign-up without a code, with an invite code, rejected code, closed sign-up, does not offer sign-up until it knows the mode, still offers it if the mode does not load, creates the account and shows field errors, refuses different passwords, does not ask to repeat the password when signing in) and add: the fields have `autocomplete` `email`, `current-password` when signing in and `new-password` (twice) when signing up, and `name` on the name; the email field is `type="email"` and the password ones `type="password"`; Enter on the last field submits the form; the button is disabled during submission.
- [ ] **Step 2: Run and see the new ones fail.**
- [ ] **Step 3: Implement** the field rows (label on the left, value on the right, in the `.field-row` style of the sheets, with the focus drawn on the row), title "gastos", a full-width main button and the mode toggle as text; field errors under the row.
- [ ] **Step 4: Run the whole suite.** Expected: green.
- [ ] **Step 5: Commit** `feat(web): sign-in and sign-up with field rows`.

---

### Task 3: Finishing touches

**Files:**
- Create: `web/src/styles/alignment.test.ts`
- Modify: `web/src/styles.css`, `web/src/styles/tokens.css`, `web/src/styles/tokens.test.ts`, `web/src/pages/Settings.tsx`, `web/src/pages/SettingsPage.test.tsx`, and the pages that still use `var(--display)` or inline title styles (`Overview.tsx`, `Transactions.tsx`, `Inbox.tsx`)

- [ ] **Step 1: Tests.** `alignment.test.ts` reads `styles.css` and fails if there is a `text-align: center` in any rule whose selector is not in the Global Constraints list; it also fails if the CSS still defines `.item`, `.item-actions`, `.inline-form`, `.inline`, `.line`, `.settings`, `.text`, `.field`, `.chip`, `.seg` or `--display`. `tokens.test.ts`: the token contract no longer requires `--display`. `SettingsPage.test.tsx`: at `/ajustes` (no section) the default detail panel shows Perfil (name and email) and is hidden on mobile by CSS; with a section open there is a single `h1` on the page (the index title becomes `h2`).
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement:** default Perfil panel on wide screens; index title as `h2` when there is a section; swap the remaining inline titles for the `.page-title` class; delete the rules and the `--display` variable (change whoever uses it to `--font`); check with `grep` in `src/` that no deleted class is used.
- [ ] **Step 4: Run the whole suite, `tsc`, `lint` and `build`.** Expected: green.
- [ ] **Step 5: Commit** `chore(web): drop the old system's CSS and guard the alignment rule`.

---

### Task 4: e2e, screenshots and wrap-up

**Files:**
- Modify: `web/e2e/app.spec.ts`

- [ ] **Step 1: Adjust the e2e tests** for Divisão and sign-up to the new controls (settlement sheet, field rows).
- [ ] **Step 2: New e2e tests:** record another settlement by hand through the sheet and see it in the history; in a wallet with splitting, at 900 px and with long names, Divisão does not scroll sideways; keyboard login (Tab to the button, Enter submits).
- [ ] **Step 3: Run** `npm run build && npm run e2e`. Expected: all pass, in both projects.
- [ ] **Step 4: Screenshots** of Divisão and of login (mobile and desktop, light and dark) with a temporary spec that is not committed; fix whatever departs from the design.
- [ ] **Step 5: Open the PR** (base: the PR 4 branch while #30 is not merged), noting that the merge triggers the deploy and requires `--admin`.
- [ ] **Step 6: Commit** `test(e2e): Divisão and sign-in in the new system`.
