# Interface restructuring, PR 2: Visão and Novo lançamento

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Visão (Overview) and the transaction sheet (new and edit) the new visual language, with the screen components that both screens need and that the next ones reuse.

**Architecture:** Five new small components (`Segmented`, `ListGroup` with `ListRow`, `Sheet`, and the visual choices `CategoryGrid`, `OptionList`, `DatePicker` inside a `PickerView`) replace the loose CSS and markup that live today in `AddSheet.tsx` and `Overview.tsx`. The sheet becomes a grouped list: tapping Categoria (Category), Quando (When) or Pago com (Paid with) swaps the sheet for a visual choice (no nested dialogs) and a Voltar (Back) button brings it back. Visão keeps all the content and the current tests, reorganized into grouped blocks.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop and Pixel 5), plain CSS with the PR 1 tokens.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), sections Screens (Visão, Novo lançamento (New transaction)), Design system and Delivery (PR 2). Base: PR 1 already on master (tokens, Geist, navigation).

## Global Constraints

- Code, comments and tests in English; interface text in pt-BR.
- Grouped blocks with radius `var(--radius-group)` (22 px) and background `var(--surface)`; controls with `var(--radius-control)` (12 px); buttons and selectors as pills; no heavy borders and **no shadows** (the only one is the floating bar's).
- Text on the left, values on the right; centered only in empty states and in the sheet's amount.
- Visão balance: 44 px (mobile) and 54 px (desktop), weight 700, tight tracking, tabular numbers.
- Touch targets of at least 44 px; visible focus; AA contrast in both themes (the tokens already pass, do not create new colors outside `tokens.css`).
- The sheet remains a native `<dialog>` (trapped focus, Esc closes); 440 px side panel on desktop, bottom-up on mobile.
- Expenses are not red: they use the normal ink. Income in `--income`. Alert (`--over`) only for budget overruns and destructive actions.
- Motion only in response to an action (sheet going up, switch), with `prefers-reduced-motion` respected.
- The API, the data and the business rules do not change; all behaviors covered today by tests in `AddSheet.test.tsx` and `Overview.test.tsx` stay covered (same behavior names, updated selectors).

## Review Focus

- Sheet focus and keyboard: Esc closes the whole sheet (even inside a choice); the Voltar button of a choice returns focus to the row that opened it. Task 3 and 5.
- Amount typed in real formats: `48,90`, `1.234,56`, `0`, empty and invalid text; only amounts greater than zero save. Task 5.
- Editing a transaction whose category is archived: the category shows up and stays selected, and the other archived ones are not offered. Task 4 and 5.
- Many categories (20 or more): the grid scrolls inside the sheet, the chosen one stays marked and the Salvar (Save) button stays visible. Task 4.
- Wallet that splits expenses: "Quem pagou" (Who paid) and "Dividir" (Split) keep working, and percentages that do not add up to 100% are refused before sending. Task 5.
- Long description (over 200 characters): the server error message shows up under the field, without closing the sheet. Task 5.
- Month with no transactions and negative balance on Visão: clear empty state and highlighted balance without using the expense red. Task 6.
- Reader: no "+" and the sheet does not open when tapping a row in the list. Task 6.

---

## File structure

| File | Responsibility |
|---|---|
| `web/src/components/Segmented.tsx` (new) | Generic segmented control (replaces `.seg`) |
| `web/src/components/ListGroup.tsx` (new) | Grouped block and list row (`ListGroup`, `ListRow`) |
| `web/src/components/Switch.tsx` (modifies) | Switch in the new style |
| `web/src/components/Sheet.tsx` (new) | Modal sheet over `<dialog>` |
| `web/src/components/pickers.tsx` (new) | `PickerView`, `CategoryGrid`, `OptionList`, `DatePicker` |
| `web/src/components/AddSheet.tsx` (modifies) | Transaction sheet as a grouped list with visual choices |
| `web/src/components/PageBar.tsx` (modifies) | `MonthSwitcher` becomes the large month title |
| `web/src/pages/Overview.tsx` (modifies) | Balance, came in and went out, "Para onde foi" (Where it went) and "Recentes" (Recent) in blocks |
| `web/src/components/TransactionList.tsx` (modifies) | Rows inside grouped blocks |
| `web/src/styles.css` (modifies) | Styles for the new components; removes `.seg`, `.chips` and old sheet rules |

---

### Task 1: Segmented control

**Files:**
- Create: `web/src/components/Segmented.tsx`, `web/src/components/Segmented.test.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces: `function Segmented<T extends string>(props: { label: string; value: T; onChange: (v: T) => void; options: { value: T; label: string }[] }): JSX.Element`. Renders `<div role="group" aria-label={label} className="segmented">` with one `<button type="button" aria-pressed>` per option.

- [ ] **Step 1: Tests in `Segmented.test.tsx`:** the group has the given name; each option is a button with `aria-pressed` true only for the current one; clicking an option calls `onChange` with its value (and clicking the already active one does not break); the options show up in the order received; ←/→ are not required (the buttons are tabbable).
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** `Segmented` and the `.segmented` CSS (background `color-mix(in srgb, var(--ink) 8%, transparent)`, radius 12, padding 3, active option on `--surface` and weight 600, minimum height 36 px).
- [ ] **Step 4: Run and see them pass.**
- [ ] **Step 5: Commit** `feat(web): segmented control`.

---

### Task 2: Grouped list and switch

**Files:**
- Create: `web/src/components/ListGroup.tsx`, `web/src/components/ListGroup.test.tsx`
- Modify: `web/src/components/Switch.tsx`, `web/src/styles.css`

**Interfaces:**
- Produces:
  - `function ListGroup(props: { children: ReactNode; className?: string }): JSX.Element`: white block with radius 22.
  - `function ListRow(props: { label: ReactNode; value?: ReactNode; icon?: ReactNode; onClick?: () => void; chevron?: boolean; trailing?: ReactNode; disabled?: boolean }): JSX.Element`: with `onClick` it renders `<button type="button" className="list-row">`, without it a `<div className="list-row">`. Visual order: icon, label, value (on the right, `--muted`), `trailing`, arrow (`›`, `aria-hidden`) when `chevron`.
  - `Switch` keeps the signature `{ checked, onChange, label, disabled }`; only the look changes.

- [ ] **Step 1: Tests in `ListGroup.test.tsx`:** a row with `onClick` is a button whose accessible name joins label and value ("Categoria Mercado") and fires the click; without `onClick` it is not a button; `chevron` does not enter the accessible name; `disabled` disables the button; `trailing` renders whatever is passed (a `Switch` works inside a row without `onClick`, and toggling calls `onChange`).
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the two components and the CSS (`.group` with vertical padding 4 and horizontal 16, `.list-row` with minimum height 48, 1 px divider in `--line` between rows, no divider on the first; `Switch` of 51x31 with the white knob and `--accent` background when on).
- [ ] **Step 4: Run and see them pass; run the whole suite** (`Switch` is already used in Recorrentes (Recurrences) and Ajustes (Settings)).
- [ ] **Step 5: Commit** `feat(web): grouped list rows and the iOS-style switch`.

---

### Task 3: Sheet

**Files:**
- Create: `web/src/components/Sheet.tsx`, `web/src/components/Sheet.test.tsx`
- Modify: `web/src/components/AddSheet.tsx` (only to use `Sheet`), `web/src/styles.css`

**Interfaces:**
- Produces: `function Sheet(props: { open: boolean; onClose: () => void; label: string; children: ReactNode }): JSX.Element`. Modal `<dialog className="sheet" aria-label={label}>`; opens with `showModal()` when `open` becomes true and closes with `close()` when false; `onClose` is called on Esc and on a click on the backdrop; the children are only rendered while `open`. Includes the handle (`.grab`), hidden on desktop.

- [ ] **Step 1: Tests in `Sheet.test.tsx`:** closed does not render the children; `open` renders and the dialog has the given name; Esc calls `onClose`; a click on the backdrop calls `onClose`, a click inside does not; when `open` goes from true to false the dialog closes. (jsdom: use `HTMLDialogElement.prototype.showModal/close` mocks in `src/test/setup.ts` if they do not exist yet.)
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement `Sheet`** extracting what `AddSheet` already does with the `<dialog>` (open and close effect, backdrop click). `AddSheet` now renders `<Sheet open={state !== null} onClose={onClose} label={...}>` with the same content as today, without changing behavior.
- [ ] **Step 4: CSS:** `--surface` background, 28 px rounded top, `::backdrop` with `--scrim`, rise animation that respects `prefers-reduced-motion`; from 900 px a 440 px side panel.
- [ ] **Step 5: Run the whole suite** (`AddSheet.test.tsx` stays green, which is the proof that nothing changed).
- [ ] **Step 6: Commit** `feat(web): Sheet component extracted from the quick-add dialog`.

---

### Task 4: Visual choices

**Files:**
- Create: `web/src/components/pickers.tsx`, `web/src/components/pickers.test.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces:
  - `function PickerView(props: { title: string; onBack: () => void; children: ReactNode }): JSX.Element`: header with a "Voltar" button (accessible name `Voltar`) and the title (`<h2>`), and the children below.
  - `function CategoryGrid(props: { categories: Category[]; value: number | null; onPick: (id: number) => void }): JSX.Element`: 3-column grid (mobile) with one button per category (emoji and name, `aria-pressed`); the grid scrolls when it is tall; marks the chosen one.
  - `function OptionList<T extends string | number>(props: { options: { value: T; label: string }[]; value: T | null; onPick: (v: T) => void }): JSX.Element`: grouped list of rows with `aria-pressed` and a checkmark on the chosen one.
  - `function DatePicker(props: { value: string; onPick: (iso: string) => void }): JSX.Element`: "Hoje" (Today) and "Ontem" (Yesterday) rows (marked if they match `value`) and a native date field with `aria-label="Data do lançamento"` for another day.

- [ ] **Step 1: Tests in `pickers.test.tsx`:** `PickerView` shows the title and `Voltar` calls `onBack`; `CategoryGrid` shows all the categories received (including 25), marks only the one in `value`, `onPick` receives the id, and an empty list does not break; `OptionList` marks the chosen one and calls `onPick` with the typed value (number or text); `DatePicker` marks Hoje when `value` is today, Ontem when it is yesterday, neither for another date (and shows the date field with that value), and choosing Ontem calls `onPick` with yesterday's date in `YYYY-MM-DD`.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the four components reusing `ListGroup` and `ListRow` from Task 2 and `today()` from `lib/dates.ts`; extract to `lib/dates.ts` a `yesterday(): string` (duplicated today inside `AddSheet.tsx`) with a test in `dates.test.ts`.
- [ ] **Step 4: Grid CSS** (`.category-grid`, `.cat-tile` with radius 16, 26 px emoji, name at 12 px, selected with an `--accent` ring, max height of 46vh with scrolling).
- [ ] **Step 5: Run and see them pass.**
- [ ] **Step 6: Commit** `feat(web): visual pickers for category, option and date`.

---

### Task 5: Transaction sheet as a grouped list

**Files:**
- Modify: `web/src/components/AddSheet.tsx`, `web/src/components/AddSheet.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Sheet`, `Segmented`, `ListGroup`, `ListRow`, `Switch`, `PickerView`, `CategoryGrid`, `OptionList`, `DatePicker`, `yesterday`.
- Produces: the same `AddSheet({ state, onClose })`; no caller changes.

Sheet layout (top to bottom): handle; `Segmented` "Tipo" (Type) (Gasto | Entrada) with the Fechar (Close) button (`aria-label="Fechar"`) beside it; large centered amount (`input` with `aria-label="Valor"`, `inputMode="decimal"`, "R$" prefix); a `ListGroup` with the rows **Categoria** (value "🛒 Mercado"), **Quando** (value "Hoje", "Ontem" or the date written out), **Pago com** (name of the method, only if any exist), **Descrição** (text field in the row itself, `aria-label="Descrição"`) and, on creation, **Repetir todo mês** (Repeat every month) (`Switch`); in a wallet that splits expenses, **Quem pagou** (choice) and **Dividir** (summary and an Ajustar/Igualar (Adjust/Equalize) button, with the percentage fields below as today); finally the full-width "Salvar" button, "Salvar e lançar outro" (Save and add another) (creation only) and, on edit, "Excluir lançamento" (Delete transaction) with confirmation.

- [ ] **Step 1: Rewrite `AddSheet.test.tsx`** keeping one test per behavior covered today (the 14 existing `it`: empty form does not send and shows what is missing; creates the expense with the default payment method and closes; switches to income and uses the income categories; "Salvar e lançar outro" keeps the sheet and clears amount and description; field error from the API; general message when the API fails; "Repetir todo mês" creates a recurrence on the date; no split in a wallet without splitting; percentages that do not add up to 100% refused and sent when they do; default split does not send `splits`; choosing who paid; edit pre-filled and saved with PUT; deletion only after confirming; no "Repetir" nor "lançar outro" on edit), swapping the chip selectors for: open the **Categoria** row, tap the category in the grid and go back. Add:
  - opening **Categoria** shows `PickerView` "Categoria" and `Voltar` goes back to the sheet with the row value updated;
  - choosing **Ontem** in Quando sends yesterday's date; **Outro dia** (Another day) with a date field sends the typed date;
  - amounts: `48,90` sends 4890, `1.234,56` sends 123456, `0` and `abc` do not send and show "Informe um valor maior que zero.";
  - editing with an archived category: the Categoria row shows the archived category and the grid only offers it and the active ones;
  - with 25 categories the grid lists all 25;
  - `Voltar` returns focus to the row that opened the choice;
  - description over 200 characters: the error returned by the API shows up and the sheet does not close.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Rewrite `AddForm`** with the structure above. New state: `picker: null | "category" | "date" | "method" | "paidBy"`; when it is not `null` the sheet shows the matching `PickerView` in place of the form (the form state is not lost); choosing closes the choice and goes back. All the logic for saving, validating, deleting and splitting stays as it is.
- [ ] **Step 4: CSS:** large amount (`font-size: 52px`, weight 700, tracking -0.04em, centered, smaller "R$" prefix in `--muted`), sheet rows with the Task 2 components, remove `.seg`, `.chips` and `.field` that end up unused (check with `grep` before deleting).
- [ ] **Step 5: Run `AddSheet.test.tsx` and the whole suite.** Expected: all green.
- [ ] **Step 6: Commit** `feat(web): quick-add sheet as a grouped list with visual pickers`.

---

### Task 6: Visão

**Files:**
- Modify: `web/src/pages/Overview.tsx`, `web/src/pages/Overview.test.tsx`, `web/src/components/PageBar.tsx`, `web/src/components/PageBar.test.tsx`, `web/src/components/TransactionList.tsx`, `web/src/components/TransactionList.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `ListGroup`, `ListRow` (Task 2).
- Produces: `MonthSwitcher()` without changing signature or accessible labels ("Mês anterior" (Previous month), "Próximo mês" (Next month)); the month becomes a large `<h1 className="month-title">` (the text is still `monthLabel(month)`, for example "janeiro de 2026", with the first letter capitalized by CSS only).

- [ ] **Step 1: Tests first.** `PageBar.test.tsx`: the month shows up as a level 1 heading and navigation to the neighbors stays the same. `Overview.test.tsx`: keep all of today's behaviors (separation of reais and cents, highlighted negative balance, daily average and previous month, empty, loading and error states, "Ver todos" (See all), month ruler, budget rails with marker and alert, suggested settlement) and add: "Para onde foi" is a block with one row per expense category (name, amount and bar), "Recentes" lists the latest 6 transactions and tapping a row opens the edit for those who can edit and does nothing for the reader; negative balance uses the `negative` class but **not** the expense color. `TransactionList.test.tsx`: the rows are still grouped by day with the day total.
- [ ] **Step 2: Run and see them fail** (the new ones; the old ones that depended on changed markup get updated in Step 3).
- [ ] **Step 3: Implement.** `MonthSwitcher` as a title; hero with a large balance, a "Entrou" and "Saiu" line with colored dots (income in `--income`, expenses in ink); "Para onde foi" in a `ListGroup` with a `ListRow` per category keeping the `rail`, `track`, `fill`, `limit` and `over` classes that the tests and the budget marker use; "Recentes" with `TransactionList` inside a `ListGroup`; the settlement card in the same block, with "Registrar acerto" as the primary action. The day ruler stays, only restyled.
- [ ] **Step 4: Desktop CSS** (from 900 px): `.hero` becomes three blocks side by side (balance, came in, went out) and the two columns "Para onde foi" and "Recentes"; on mobile, one column.
- [ ] **Step 5: Run `npx vitest run`, `tsc`, `npm run lint`, `npm run build`.** Expected: all green.
- [ ] **Step 6: Commit** `feat(web): overview in grouped blocks with the large month title`.

---

### Task 7: e2e, screenshots and wrap-up

**Files:**
- Modify: `web/e2e/app.spec.ts`

- [ ] **Step 1: Adjust the e2e tests** that use the old sheet: the `addExpense` helper now opens Categoria (grid), chooses and goes back, instead of clicking a chip; the "Repetir todo mês" flow uses the row with the switch; the ones that edit and delete keep the same labels ("Salvar", "Excluir lançamento", "Confirmar exclusão" (Confirm deletion)).
- [ ] **Step 2: New e2e test "quick add":** in both projects, open "Novo lançamento", type `48,90`, choose Categoria and Quando (Ontem), save, and see the transaction in Lançamentos (Transactions) with yesterday's date; open again and confirm that Esc closes the sheet without saving.
- [ ] **Step 3: Run** `npm run build && npm run e2e`. Expected: all pass.
- [ ] **Step 4: Screenshots** (mobile and desktop, light and dark, Visão and the sheet) with a temporary spec that is **not** committed; check by eye and fix whatever departs from the approved mockup before the commit.
- [ ] **Step 5: Open the PR** describing what changed and that the merge triggers the deploy; the merge requires the user's approval and today `--admin`.
- [ ] **Step 6: Commit** `test(e2e): quick add with visual pickers`.
