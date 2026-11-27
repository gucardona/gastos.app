# Interface restructuring, PR 4: Ajustes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild Ajustes (Settings) as two short lists where each row opens a screen of its own, with visual icon and color pickers, and sheets in place of loose forms.

**Architecture:** `/ajustes` becomes an index (grouped lists) and `/ajustes/:section` the screen for each subject. On mobile the index and the screen alternate, with a "‹ Ajustes" link to go back; from 900 px they show up side by side (list on the left, content on the right). Each section swaps the inline form for list rows and a sheet (`Sheet`) for creating and editing, reusing `Segmented`, `ListGroup`, `Switch` and the pickers from PR 2. Two new components come in: `EmojiPicker` (grid with tabs by subject) and `ColorSwatches` (row of swatches). Recorrentes (Recurrences) also gets a screen in Ajustes, reusing `RecurringSection`.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop and Pixel 5), plain CSS with the PR 1 tokens.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), section Ajustes (Pessoas (People), Categorias (Categories), Contas bancárias (Bank accounts), Aparência (Appearance)), Design system and Delivery (PR 4). Base: PRs 1 to 3 on master.

## Global Constraints

- Code, comments and tests in English; interface text in pt-BR.
- Same system as PRs 2 and 3: grouped blocks with radius 22, controls with radius 12, no shadows, text on the left and values on the right, touch targets of at least 44 px, visible focus, AA contrast in both themes, and only tokens from `tokens.css`. The category color swatches are user data (a fixed palette of 8 colors defined in the component), not interface tokens.
- Ajustes index, in this order. Group with no title: **Perfil** (Profile). Group "Casa" (Home): **Carteira** (Wallet), **Pessoas**, **Categorias**, **Formas de pagamento** (Payment methods), **Orçamentos** (Budgets). Group "Dados" (Data): **Contas bancárias**, **Recorrentes**, **Aparência**. Last, **Sair** (Sign out) (text in the alert color, no confirmation, works even offline as today).
- Routes: `/ajustes` and `/ajustes/:section` with `section` in `perfil`, `carteira`, `pessoas`, `categorias`, `pagamentos`, `orcamentos`, `bancos`, `recorrentes`, `aparencia`. The old address `/ajustes?s=<id>` redirects (`conta` to `perfil`, `membros` to `pessoas`; the other ids are the same). Unknown section goes back to `/ajustes`.
- Expense type with the spec's labels: Essencial | Supérfluo | Investimento (API values: `essential`, `nonessential`, `investment`).
- Reader and whoever does not administer: everything visible and nothing editable, as today (each write control disappears or is disabled according to the role).
- Deletions ask for confirmation in the same place; deleting a category that has transactions archives it and explains ("Arquivada, porque já tem lançamentos." (Archived, because it already has transactions.)).
- The API, the data and the rules do not change. Every behavior covered today by `Settings.test.tsx`, `SettingsPage.test.tsx` and `BankSection.test.tsx` stays covered (same behaviors, updated selectors).

## Review Focus

- Whoever had `/ajustes?s=bancos` saved (and the link from the empty Caixa (Inbox)) lands on the bank accounts screen; unknown id does not leave a blank screen. Task 1.
- Edited category whose icon is not in the grid (typed freely in the old version): the icon shows up selected and is not lost on save. Task 2 and 3.
- Archived category or payment method: shows up under "Arquivadas" (Archived), restores, and deleting one that is in use becomes archiving with the message. Task 3.
- Whoever is not an administrator: does not invite, does not remove, but can leave the wallet; a reader edits nothing in any section. Task 4.
- Budget: invalid or zero amount is refused before sending; removing clears the field. Task 4.
- Wallet: creating a new one starts using it; deleting only after confirming, with the warning when it is the only one. Task 4.
- Bank accounts: importing OFX and CSV (column mapping only for CSV), disabled account does not import, rules listed and removable. Task 5.
- No horizontal scroll between 900 and 1100 px in any section, with long names (category, account, email). Task 6.

---

## File structure

| File | Responsibility |
|---|---|
| `web/src/pages/Settings.tsx` (rewrites) | Index, master and detail layout, redirect from the old address, Perfil, Aparência and Sair |
| `web/src/pages/settings/` (modifies) | One screen per subject, each with its sheet: `WalletSection`, `MembersSection`, `CategoriesSection`, `PaymentsSection`, `BudgetsSection`, `BankSection` |
| `web/src/components/EmojiPicker.tsx` (new) | Emoji grid with tabs by subject |
| `web/src/components/ColorSwatches.tsx` (new) | Row of color swatches |
| `web/src/pages/Inbox.tsx` (modifies) | The empty Caixa link points to `/ajustes/bancos` |
| `web/src/styles.css` (modifies) | New styles; removes `.item`, `.item-actions`, `.inline-form`, `.settings`, `.line`, `.seg`, `.chip` and `.filters` when `grep` shows nothing else uses them |

---

### Task 1: Index, routes and simple screens

**Files:**
- Modify: `web/src/pages/Settings.tsx`, `web/src/pages/SettingsPage.test.tsx`, `web/src/App.tsx`, `web/src/pages/Inbox.tsx`, `web/src/pages/Inbox.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `ListGroup`, `ListRow`, `Segmented`, `RecurringSection`, `PageBar`.
- Produces: `function Settings(): JSX.Element` (layout), `export const SETTINGS_SECTIONS` with `{ id, label, group }` for the nine sections; each section's screens remain `*Section` components without props, rendered by the layout according to `useParams().section`.

- [ ] **Step 1: Tests** in `SettingsPage.test.tsx`: `/ajustes` shows the three lists in the order from the Global Constraints, with the name and email in Perfil as the value, and "Sair" last; each row leads to `/ajustes/<id>` (`href`); opening `/ajustes/aparencia` shows the `Segmented` "Tema" (Theme) (Automático, Claro, Escuro) that applies and remembers the choice (same behaviors as today); `/ajustes/perfil` shows name, email and the role in the wallet; `/ajustes/recorrentes` shows the "Repetem todo mês" (Repeat every month) section; `/ajustes?s=bancos` redirects to `/ajustes/bancos`, `?s=conta` to `/ajustes/perfil`, `?s=membros` to `/ajustes/pessoas`; `/ajustes/inexistente` redirects to `/ajustes`; in any section there is the "‹ Ajustes" link to go back; Sair ends the session and leads to the login, also when the call fails. In `Inbox.test.tsx`, the empty inbox link has `href="/ajustes/bancos"`.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the layout: the index and the detail side by side from 900 px (the active row highlighted), alternating on mobile; Perfil, Aparência (with `Segmented`) and Sair in `Settings.tsx` itself; the sections not yet migrated (`carteira`, `pessoas`, `categorias`, `pagamentos`, `orcamentos`, `bancos`) keep the current content inside the new layout. Update the Caixa link and the route (`<Route path="ajustes/:section?" ...>`).
- [ ] **Step 4: Run the whole suite** (`vitest`, `tsc`, `lint`). Expected: green.
- [ ] **Step 5: Commit** `feat(web): Ajustes as an index with a screen per subject`.

---

### Task 2: Icon and color pickers

**Files:**
- Create: `web/src/components/EmojiPicker.tsx`, `web/src/components/EmojiPicker.test.tsx`, `web/src/components/ColorSwatches.tsx`, `web/src/components/ColorSwatches.test.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces:
  - `function EmojiPicker(props: { value: string; onPick: (emoji: string) => void }): JSX.Element`: tabs Casa (Home), Comida (Food), Transporte (Transport), Lazer (Leisure), Saúde (Health), Dinheiro (Money) and Outros (Other) (buttons with `aria-pressed`), and a 6-column grid of buttons whose accessible name is the emoji itself. Shows the tab that contains `value`; if `value` is in no tab, it shows up as the first option of the grid, marked.
  - `function ColorSwatches(props: { value: string; onPick: (hex: string) => void }): JSX.Element`: 8 swatches (`#0E6577`, `#3B6FD8`, `#7A5AD6`, `#D4568B`, `#D9822B`, `#2F9D68`, `#8B949A`, `#C2412D`) as `aria-pressed` buttons with the name `Cor <hex>` (Color); if `value` is not one of them (old color), it comes in as a ninth, marked swatch.

- [ ] **Step 1: Tests:** `EmojiPicker` opens on the tab of `value` (e.g. does 🛒 open on Comida? define the mapping in the test with the real emojis of the tabs), switching tabs shows its emojis, `onPick` receives the tapped emoji, only `value` is `aria-pressed`, and an emoji outside the tabs shows up marked as the first option; each tab has at least 12 emojis and no emoji repeats between tabs (data guard). `ColorSwatches`: 8 swatches, only the one in `value` marked, `onPick` receives the hex, and a color outside the palette becomes the ninth, marked swatch.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the two components and the CSS (round 32 px swatch with a 44 px touch area, `--accent` ring on the marked one; tabs in discreet horizontal scrolling; 6-column grid with 44 px buttons).
- [ ] **Step 4: Run and see them pass.**
- [ ] **Step 5: Commit** `feat(web): emoji picker and color swatches`.

---

### Task 3: Categories and Payment methods

**Files:**
- Create: `web/src/components/CategorySheet.tsx`, `web/src/components/CategorySheet.test.tsx`, `web/src/components/PaymentSheet.tsx`, `web/src/components/PaymentSheet.test.tsx`
- Modify: `web/src/pages/settings/CategoriesSection.tsx`, `web/src/pages/settings/PaymentsSection.tsx`, `web/src/pages/settings/Settings.test.tsx`

**Interfaces:**
- Consumes: `Sheet`, `Segmented`, `ListGroup`, `ListRow`, `EmojiPicker`, `ColorSwatches`, `useAction`.
- Produces:
  - `function CategorySheet(props: { category: Category | "new" | null; kind: Kind; onClose: () => void }): JSX.Element`: closed with `null`; `"new"` creates in the category of type `kind`; a `Category` edits. Content: preview (large icon and name), `Nome` (Name) field, `EmojiPicker`, `ColorSwatches`, and for expenses the `Segmented` "Tipo de gasto" (Expense type) (Essencial, Supérfluo, Investimento); buttons Salvar (Save) (or Criar categoria (Create category)), and on edit Arquivar (Archive) or Restaurar (Restore) and "Excluir categoria" with confirmation.
  - `function PaymentSheet(props: { method: PaymentMethod | "new" | null; onClose: () => void }): JSX.Element`: same pattern, with `Nome` and `EmojiPicker`.

The Categorias screen: `Segmented` "Tipo de categoria" (Category type) (Gastos | Entradas), a `ListGroup` with one row per active category (icon, name, expense type as the value, color swatch), a "Nova categoria" (New category) button and the collapsible section "Arquivadas (N)". Tapping a row opens `CategorySheet`; a reader sees the rows without opening anything.

- [ ] **Step 1: Tests** porting the behaviors of `Settings.test.tsx` (categories: creates an expense with expense type, income without expense type, shows the API error as duplicate name, explains when the deletion became archiving, restores archived; payment methods: creates one) and adding: editing sends PUT with the complete body (`kind`, `name`, `icon`, `color`, `essentiality`, `sortOrder`, `archived`); an icon outside the grid stays selected and is sent on save; empty name is refused by the API and the error shows up without closing the sheet; a reader does not open the sheet; the list shows the category color.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the two sheets and rewrite the two screens (the `body()` logic and the deletion that archives stay).
- [ ] **Step 4: Run the whole suite.** Expected: green.
- [ ] **Step 5: Commit** `feat(web): categories and payment methods with sheets and visual pickers`.

---

### Task 4: People, Wallet and Budgets

**Files:**
- Create: `web/src/components/MemberSheet.tsx`, `web/src/components/MemberSheet.test.tsx`, `web/src/components/BudgetSheet.tsx`, `web/src/components/BudgetSheet.test.tsx`
- Modify: `web/src/pages/settings/MembersSection.tsx`, `web/src/pages/settings/WalletSection.tsx`, `web/src/pages/settings/BudgetsSection.tsx`, `web/src/pages/settings/Settings.test.tsx`

**Interfaces:**
- Produces:
  - `function MemberSheet(props: { member: Member | null; onClose: () => void }): JSX.Element`: for the administrator, `Segmented` "Permissão" (Permission) (Pode editar | Só leitura (Can edit | Read only)) of another member, `Switch` "Dividir gastos com <nome> por padrão" (Split expenses with <name> by default) when the wallet splits, and remove; for any non-administrator member, "Sair da carteira" (Leave the wallet) (only on their own row). Confirmation on both exits.
  - `function BudgetSheet(props: { category: Category | null; current?: number; onClose: () => void }): JSX.Element`: field `Limite mensal de <categoria>` (Monthly limit of <category>), Definir (Set) and, if there is a limit, Remover (Remove).

Screens: **Pessoas** (members in a list; tapping opens `MemberSheet`; for the administrator a "Convidar alguém" (Invite someone) row opens a sheet with `E-mail do convidado` (Invitee email), `Segmented` "Permissão do convidado" (Invitee permission) and the Convidar (Invite) button); **Carteira** (row `Nome da carteira` (Wallet name) editable only by the administrator with "Salvar nome" (Save name), `Switch` "Dividir gastos entre os membros" (Split expenses between members), "Nova carteira" (New wallet) row that opens a sheet with a name and the switch, and "Excluir esta carteira" (Delete this wallet) in the alert color with confirmation and the warning for the only wallet); **Orçamentos** (one row per active expense category, the limit as the value; tapping opens `BudgetSheet`).

- [ ] **Step 1: Tests** porting the behaviors of `Settings.test.tsx` (members: invites by email with the chosen permission, shows the error when the email has no account, the administrator changes another member's permission but not their own, whoever does not administer does not invite or remove but can leave; budgets: shows the limit and saves in cents, refuses invalid amount without calling the API, removes the limit, lists only active expense categories; wallet: turns on splitting, creates a new one and starts using it, deletes only after confirming, whoever does not administer does not edit) through the new sheets, plus: the permission `Segmented` sends `PATCH` with `{ role }`; the default split switch sends `{ splitsByDefault }`; confirming leaving the wallet sends `DELETE` for the member themselves; zero and text in the limit show "Informe um valor maior que zero.".
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the two sheets and rewrite the three screens.
- [ ] **Step 4: Run the whole suite.** Expected: green.
- [ ] **Step 5: Commit** `feat(web): people, wallet and budgets with sheets`.

---

### Task 5: Bank accounts

**Files:**
- Create: `web/src/components/BankAccountSheet.tsx`, `web/src/components/BankAccountSheet.test.tsx`
- Modify: `web/src/pages/settings/BankSection.tsx`, `web/src/pages/settings/BankSection.test.tsx`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `Switch`, `OptionList`, `PickerView`, `Segmented`, `useAction`, `api.upload`.
- Produces: `function BankAccountSheet(props: { account: BankAccount | "new" | null; onClose: () => void }): JSX.Element`: new (name only) or existing (payment method of the transactions as a row that opens `OptionList`, `Switch` "Conta ativa" (Active account), **Importar extrato** (Import bank statement) with the format in a `Segmented` (OFX | CSV), the file and, only for CSV, the date, description and amount columns, the header and "valores positivos são compras" (positive values are purchases), and "Excluir conta" (Delete account) with confirmation).

The screen: list of accounts (name, "N para revisar" (N to review), "em dia" (up to date) or "desativada" (disabled)), "Nova conta bancária" (New bank account), and below "Regras de categoria" (Category rules) (each rule with the text and the category; remove with one tap). Importing shows "N linhas novas, M já importadas antes." (N new rows, M already imported before.) as today.

- [ ] **Step 1: Tests** porting the 8 from `BankSection.test.tsx` (list with what is waiting for review; creates an account; sends OFX and reports what was new; asks for the CSV columns and sends them; does not import without a file; shows why the file was refused; changes the payment method keeping the other fields; deletes a rule) through the sheet, and adding: a disabled account does not offer importing; the CSV format shows the column fields and OFX does not; removing the account only after confirming.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement** the sheet and the screen (the sending logic, `FormData` and the messages stay).
- [ ] **Step 4: Run the whole suite.** Expected: green.
- [ ] **Step 5: Commit** `feat(web): bank accounts with a sheet and the import form inside it`.

---

### Task 6: e2e, cleanup and wrap-up

**Files:**
- Modify: `web/e2e/app.spec.ts`, `web/src/styles.css`

- [ ] **Step 1: Adjust the e2e tests** that use Ajustes: the "settings and recurring" flow (new category with name, grid icon, color and expense type; set a budget; see the overrun in Visão (Overview)), the OFX one (create the account and import through the sheet), the sign-out one ("Sair" row) and the navigation one (a single "Ajustes").
- [ ] **Step 2: New e2e tests:** opening `/ajustes?s=bancos` leads to `/ajustes/bancos`; inviting a person through the sheet shows the error when the email does not exist; and, at 900 px with long names (category, bank account), no section scrolls sideways (same pattern as PRs 2 and 3).
- [ ] **Step 3: CSS cleanup:** delete the classes that `grep` shows as unused (`.item`, `.item-actions`, `.inline-form`, `.settings`, `.line`, `.seg`, `.chip`, `.filters`, `.swatch` and the like), checking each one in `src/` first.
- [ ] **Step 4: Run** `npm run build && npm run e2e`. Expected: all pass, in both projects.
- [ ] **Step 5: Screenshots** of the index and of three sections (Categorias with the sheet, Pessoas, Contas bancárias), mobile and desktop, light and dark, with a temporary spec that is not committed; fix whatever departs from the design.
- [ ] **Step 6: Open the PR**, noting that the merge triggers the deploy and today requires `--admin`.
- [ ] **Step 7: Commit** `test(e2e): the new Ajustes`.
