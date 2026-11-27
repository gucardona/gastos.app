# Interface restructuring, PR 1: foundation and navigation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the visual foundation (tokens, font) and the navigation (floating bar on mobile, sidebar on desktop, "Ajustes" (Settings) only through the avatar) without breaking any existing screen.

**Architecture:** The current tokens (`--paper`, `--surface`, `--ink`, `--muted`, `--line`, `--accent`...) keep their names and get the new values, so the old screens inherit the new look without a rewrite. Navigation moves out of `Shell.tsx` into two components (`TabBar`, `Sidebar`) that share the list of destinations in `nav.ts`. The screen components (`Segmented`, `ListRow`, `Sheet` etc.) arrive in the PR that uses them first, not here.

**Tech Stack:** React 19, React Router 8, Vite, Vitest + Testing Library, Playwright (desktop and Pixel 5), `@fontsource-variable/geist`.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), sections Navigation, Design system and Delivery (PR 1).

## Global Constraints

- Code, comments and tests in English; interface text in pt-BR.
- Light colors: background `#F2F3F4`, surface `#FFFFFF`, ink `#14181B`, secondary `#7B858B`, line `#ECEEEF`, accent `#0E6577`, income `#1E8E5A`, alert `#C2412D`.
- Dark colors: background `#0F1113`, surface `#191C1F`, ink `#F3F4F5`, secondary `#8A949A`, line `#23272A`, accent `#6CC4D6`, income `#5BD08D`, alert `#FF8A78`. They follow `prefers-color-scheme`; `data-theme="light|dark"` forces one.
- Single font: Geist (variable), served by the app itself via `@fontsource-variable/geist`, without asking third parties. Tabular numbers.
- Radii: groups 22 px, controls 12 px, buttons and selectors as pills. Shadow only on the floating bar.
- Destinations: Visão (Overview) `/`, Lançamentos (Transactions) `/lancamentos`, Caixa (Inbox) `/caixa`, Divisão (Split) `/divisao` (only when the wallet splits expenses), plus "+" that opens Novo lançamento (New transaction) (only for those who can edit).
- Ajustes: a single path, the avatar (`/ajustes`). Recorrentes (Recurrences) leaves the navigation.
- Sidebar from 900 px; below that, floating bar. Touch targets of at least 44 px, visible focus, `aria-current="page"` on the active destination, respect `env(safe-area-inset-bottom)`.
- The API, the data and the inner screens do not change in this PR.

## Review Focus

- Wallet without splitting: the Divisão tab does not exist (and the bar has three tabs plus the "+"). Tested in Task 2 and Task 3.
- Reader (`canEdit` false): no "+" or "Novo lançamento", neither in the bar nor in the sidebar. Task 2 and 3.
- Caixa counter: hidden at 0, exact number up to 99, "99+" above that. Task 2.
- Recorrentes is still reachable after leaving the menu (a link in Lançamentos). Task 4.
- Complete dark mode: every color role defined in both themes, including `--tick` and `--tick-future`, which the Visão ruler uses. Task 1.
- Only one visible path to Ajustes per viewport, and the active wallet is still switchable on desktop after hiding the `PageBar` there. Task 3 and 5.

---

## File structure

| File | Responsibility |
|---|---|
| `web/src/styles/tokens.css` (new) | Color, font and radius variables, light and dark |
| `web/src/styles.css` (modifies) | Imports the tokens, loses the old `:root` block, gains the CSS for `.tabbar`, `.sidebar`; removes the old `.bottom`, `.fab` |
| `web/src/main.tsx` (modifies) | Imports Geist in place of Bricolage and Instrument Sans |
| `web/src/components/Icon.tsx` (new) | Stroke icons: `home`, `list`, `tray`, `split`, `plus` |
| `web/src/components/nav.ts` (new) | List of destinations and `formatBadge` |
| `web/src/components/TabBar.tsx` (new) | Floating bar on mobile |
| `web/src/components/Sidebar.tsx` (new) | Desktop sidebar, with wallet and profile |
| `web/src/Shell.tsx` (modifies) | Uses `TabBar` and `Sidebar`; no navigation list of its own |
| `web/src/pages/Transactions.tsx` (modifies) | "Repetem todo mês" (Repeat every month) link to `/recorrentes` |

---

### Task 1: Tokens and font

**Files:**
- Create: `web/src/styles/tokens.css`
- Create: `web/src/styles/tokens.test.ts`
- Modify: `web/src/styles.css` (removes `:root` and the two dark blocks from lines 1 to 14, adds `@import "./styles/tokens.css";` on the first line)
- Modify: `web/src/main.tsx`, `web/package.json`

**Interfaces:**
- Produces: CSS variables `--paper --surface --ink --muted --line --accent --on-accent --income --over --scrim --tick --tick-future --font --ui --display --radius-group --radius-control`. `--ui` and `--display` both point to `--font` (the names stay so that we do not touch every rule; `--display` is removed in PR 5).

- [ ] **Step 1: Write the token contract test** in `tokens.test.ts`: reads `tokens.css` as text and, for each of the three contexts (`:root`, `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"])`, `:root[data-theme="dark"]`), confirms that the block defines `--paper`, `--surface`, `--ink`, `--muted`, `--line`, `--accent`, `--on-accent`, `--income`, `--over`, `--scrim`, `--tick`, `--tick-future`. It also confirms the exact values from the Global Constraints for `--paper`, `--accent` and `--over` in both themes, and that the dark block and `[data-theme="dark"]` have identical values.
- [ ] **Step 2: Run `npx vitest run src/styles` and see it fail** (file does not exist).
- [ ] **Step 3: Create `tokens.css`** with the values from the Global Constraints. `--tick`: `#B9C0C4` (light) and `#5A646A` (dark); `--tick-future`: `#D9DDE0` and `#2A2F33`; `--scrim`: `rgba(20,24,27,.28)` and `rgba(0,0,0,.55)`; `--font: "Geist Variable", system-ui, sans-serif`; `--radius-group: 22px`; `--radius-control: 12px`; `color-scheme` per theme.
- [ ] **Step 4: Swap the font.** `npm remove @fontsource-variable/bricolage-grotesque @fontsource-variable/instrument-sans && npm i @fontsource-variable/geist`; in `main.tsx`, import `@fontsource-variable/geist` in place of the two old ones. In `styles.css`, `body` uses `font: 400 15px/1.45 var(--ui)` (already does) and `.balance`, `.auth h1`, `.settle .big`, `.amount input` keep using `var(--display)`.
- [ ] **Step 5: Run everything** (`npx vitest run`, `npm run lint`, `npx tsc --noEmit`, `npm run build`). Expected: all green, and `dist` loads a single Geist font file.
- [ ] **Step 6: Commit** `feat(web): new design tokens and the Geist typeface`.

---

### Task 2: Icons, destinations and floating bar

**Files:**
- Create: `web/src/components/Icon.tsx`, `web/src/components/nav.ts`, `web/src/components/TabBar.tsx`
- Test: `web/src/components/TabBar.test.tsx`, `web/src/components/nav.test.ts`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces:
  - `type IconName = "home" | "list" | "tray" | "split" | "plus"`; `function Icon({ name }: { name: IconName }): JSX.Element` (SVG 24x24, stroke 1.7, `aria-hidden`).
  - `type NavItem = { to: string; label: string; icon: IconName; end?: boolean }`; `function navItems(splitting: boolean): NavItem[]` returns Visão, Lançamentos, Caixa and, only if `splitting`, Divisão, in that order.
  - `function formatBadge(n: number): string` returns `""` for `n <= 0`, `String(n)` up to 99 and `"99+"` above that.
  - `function TabBar({ pending, onAdd }: { pending: number; onAdd: () => void }): JSX.Element`; reads `wallet.splittingEnabled` and `canEdit` from `useApp()`.

- [ ] **Step 1: Tests for `nav.test.ts`:** `navItems(false)` has exactly the labels `["Visão", "Lançamentos", "Caixa"]`; `navItems(true)` adds `"Divisão"` last; no item points to `/ajustes` or `/recorrentes`; `formatBadge` of `0`, `-1` → `""`, `4` → `"4"`, `99` → `"99"`, `100` and `250` → `"99+"`.
- [ ] **Step 2: Tests for `TabBar.test.tsx`** (with `renderApp`): the `nav` "Navegação" (Navigation) has the links in order, with the correct `href`; a wallet without splitting does not show "Divisão" and with splitting it does; the "Novo lançamento" button calls `onAdd` on click and disappears when `canEdit` is false; with `pending=4` the Caixa link contains "4", with `0` it contains no number, with `250` it shows "99+"; at `route="/lancamentos"` only Lançamentos has `aria-current="page"`.
- [ ] **Step 3: Run and see them fail.**
- [ ] **Step 4: Implement** `Icon.tsx` (a `switch` by name, paths from the approved mockup), `nav.ts` and `TabBar.tsx` (`NavLink` with `end` on Visão; the counter is a `<b className="badge">` inside the Caixa link, with no `aria-label`).
- [ ] **Step 5: CSS in `styles.css`:** `.tabbar` fixed 16 px from the bottom (plus `env(safe-area-inset-bottom)`), with the `.tabbar-pill` pill (height 58, translucent background with `backdrop-filter: blur(14px)`, the only shadow in the system) and the round 58 px `.fab` next to it, in `--accent`. Active item in `--accent`, weight 600. Removes the `.bottom` rules and the old `.fab`. Hides `.tabbar` from 900 px.
- [ ] **Step 6: Run the new tests and see them pass.**
- [ ] **Step 7: Commit** `feat(web): floating tab bar, icons and shared nav items`.

---

### Task 3: Sidebar and Shell

**Files:**
- Create: `web/src/components/Sidebar.tsx`, `web/src/components/Sidebar.test.tsx`
- Modify: `web/src/Shell.tsx`, `web/src/Shell.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `navItems`, `formatBadge`, `Icon` (Task 2).
- Produces: `function Sidebar({ pending, onAdd }: { pending: number; onAdd: () => void }): JSX.Element`.

- [ ] **Step 1: Tests for `Sidebar.test.tsx`:** same battery as `TabBar` (link order, conditional Divisão, counter, `aria-current`, "Novo lançamento" that calls `onAdd` and disappears for a reader); plus: there are no "Recorrentes" or "Ajustes" links among the destinations; there is a profile link with the user's name, `href="/ajustes"` and accessible name "Ajustes"; the `Carteira` (Wallet) selector shows the active wallet and, when changed, calls `setWalletId` with the id; it shows "com Bia" (with Bia) when there is another person.
- [ ] **Step 2: Update `Shell.test.tsx`:** where it now looks for "Recorrentes" or "Ajustes" in the navigation, expect the four destinations instead; a new test confirms that with `splittingEnabled: false` the `Shell` does not render "Divisão" in either bar.
- [ ] **Step 3: Run and see them fail.**
- [ ] **Step 4: Implement `Sidebar.tsx`** (an `<aside className="sidebar" aria-label="Navegação">`: wallet selector at the top, destinations, space, "Novo lançamento" button, profile in `<Link to="/ajustes" aria-label="Ajustes">`). **In `Shell.tsx`:** delete `NAV` and the JSX for `aside` and `nav.bottom`, render `<Sidebar pending onAdd />` and `<TabBar pending onAdd />`; keep the `N` shortcut.
- [ ] **Step 5: CSS:** `.sidebar` fixed at 248 px from 900 px, background slightly darker than `--paper`, active item in a white block (`--surface`) with `--accent`; `.main` gets the side space. Hides `.bar` (the `PageBar`) from 900 px, because the wallet and the profile now live in the sidebar. The bottom padding of `.main` on mobile keeps the content clear of the floating bar.
- [ ] **Step 6: Run `npx vitest run` and see everything pass.**
- [ ] **Step 7: Commit** `feat(web): sidebar and the new shell navigation`.

---

### Task 4: Recorrentes stays reachable

**Files:**
- Modify: `web/src/pages/Transactions.tsx`, `web/src/pages/Transactions.test.tsx`

**Interfaces:**
- Consumes: route `/recorrentes` (exists, does not change).

- [ ] **Step 1: Test in `Transactions.test.tsx`:** the page shows a "Repetem todo mês" link with `href="/recorrentes"`, for those who can edit and for readers.
- [ ] **Step 2: Run and see it fail.**
- [ ] **Step 3: Implement** the link right below the title, as accent text with an arrow (`›`), temporary until PR 3 moves the section into the list.
- [ ] **Step 4: Run and see it pass.**
- [ ] **Step 5: Commit** `feat(web): keep recurrences reachable from Lançamentos`.

---

### Task 5: e2e, screenshots and wrap-up

**Files:**
- Modify: `web/e2e/app.spec.ts`
- Modify: `docs/superpowers/specs/2026-10-11-ui-redesign-design.md` (Delivery, item 1)

- [ ] **Step 1: Adjust the existing e2e tests:** where they reach Recorrentes through the menu (`getByRole("link", { name: /Recorrentes/ })`), go through Lançamentos and "Repetem todo mês". Where they click "Ajustes", keep using `getByRole("link", { name: "Ajustes" }).first()`.
- [ ] **Step 2: New e2e test "navigation":** in both projects, after signing up, the navigation has Visão, Lançamentos and Caixa visible; Divisão does not exist in a wallet without splitting; exactly one "Ajustes" link is visible; "Novo lançamento" opens the sheet; on desktop the active wallet shows up in the sidebar.
- [ ] **Step 3: Run the suite** (`npm run build && npm run e2e`) on desktop and mobile. Expected: all pass.
- [ ] **Step 4: Update the spec:** in the Delivery section, item 1 now says "tokens, font and the new navigation; the screen components arrive together with the first screen that uses them".
- [ ] **Step 5: Open the PR** with screenshots of mobile (light and dark) and desktop in the description. Merge only with the user's approval (it triggers the deploy).
- [ ] **Step 6: Commit** `test(e2e): navigation, and the spec note on component timing`.
