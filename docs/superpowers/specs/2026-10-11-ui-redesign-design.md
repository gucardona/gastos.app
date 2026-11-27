# Interface restructuring: minimalist, Apple style

Date: 2026-10-11. Replaces, for the visual and navigation layer, the design in [2026-10-09-frontend-rewrite-design.md](2026-10-09-frontend-rewrite-design.md). The API, the data and the business rules do not change.

## Intent

The app is used by two people (the household: Gustavo and Bia) to log expenses fast and understand where the money went. The current version works, but it is confusing: Ajustes (Settings) shows up in two places, forms are stacks of fields with no hierarchy, a category icon is an empty field asking for "um emoji" (an emoji), and text alignment varies from screen to screen.

Success means:

- **Minimalist and intuitive, extremely easy to use.** Each thing has a single path. Nobody needs instructions to log an expense, accept a bank statement line or invite someone.
- **Visual reference: Apple products.** Few elements, lots of space, large titles, grouped lists, controls that explain themselves.
- **The number is the protagonist.** What the user wants to know (how much is left, where it went) carries the most weight on the screen.

Chosen by the user among five mocked directions: cool gray, rounded white blocks, large number, floating navigation bar with a separate "+". The discarded alternatives (graphite, typographic, ring, colored blocks) are out of scope; dark mode comes in as a variant of the same system, not as its own direction.

## Out of scope

Google login, Open Finance (Pluggy) and any API or data model change. They come later, already on the new system. Language and currency stay fixed at pt-BR and real, with no configuration.

## Navigation

Four destinations and one action:

| Destination | Content |
|---|---|
| Visão (Overview) | month balance, where it went, recent |
| Lançamentos (Transactions) | month list by day, search, "Repetem todo mês" (Repeat every month) section |
| Caixa (Inbox) | imported bank statement lines waiting for a decision |
| Divisão (Split) | settlements between people (only when the wallet splits expenses) |
| + | new transaction (sheet) |

Rules:

- **Ajustes only through the avatar** (top right corner on mobile, bottom of the sidebar on desktop). The gear and the current duplication go away.
- **Recurrences is not a destination.** It lives as a section inside Lançamentos and as a list in Ajustes.
- **Active wallet** at the top of Visão (name and "com Bia" (with Bia)); tapping it opens the wallet switcher.
- **Mobile:** translucent floating bar with four tabs and the "+" as a round button on the right, separate. **From 900 px:** 248 px sidebar, with "Novo lançamento" (New transaction) as a button and the avatar at the bottom. The Divisão tab only shows up when the wallet splits expenses.
- The Caixa shows a pending counter; it reads as a number, with no extra text.

## Screens

**Visão.** Month title ("Outubro") with discreet ‹ › arrows. Below, the balance in a large font, and a line with "Entrou" (Came in) and "Saiu" (Went out) (colored dots, values in medium weight). Then "Para onde foi" (Where it went): categories with a thin proportional bar, in a grouped block. On desktop, three numbers side by side (balance, came in, went out) and two columns (Para onde foi, Recentes (Recent)).

**New transaction / edit (sheet).** Segmented control Gasto | Entrada (Expense | Income); large amount in the center; grouped list with Categoria, Quando, Pago com, Descrição and the "Repetir todo mês" switch; a full-width "Salvar" (Save) button. Categoria, Quando and Pago com open a visual choice (category grid, calendar, list), not a keyboard. On split transactions, a "Dividir" row opens the parts. Delete sits at the end of the edit sheet, with confirmation.

**Lançamentos.** Month title, search, list grouped by day (date on the left, day total on the right). Tapping a row opens the edit sheet. The "Repetem todo mês" section lists the recurrences with a switch to pause and the row opens the edit.

**Caixa.** Top with "Aceitar N sugeridas" (Accept N suggested) when there are any. Each row shows description, amount and account; tapping opens a sheet with the category (grid), the optional rule text ("Usar nas próximas importações" (Use on the next imports)), "Aceitar", "É este lançamento" (This is that transaction) (when there is a candidate) and "Ignorar". Empty: explains how to import and links to Ajustes, Contas bancárias (Bank accounts).

**Divisão.** A large sentence ("Bia deve R$ 120,00 a você" (Bia owes you R$ 120,00)) and the "Registrar acerto" (Record settlement) action; history below.

**Ajustes.** Two short lists, each row with an arrow and its own destination: Perfil; Pessoas, Categorias, Formas de pagamento, Orçamentos; Contas bancárias, Recorrentes, Aparência; Sair (red, at the end). On desktop it is master and detail: the list on the left, the content on the right.
- **Pessoas (People):** list with the permission as a segmented control (Pode editar | Só leitura (Can edit | Read only)); inviting is an email field, the segmented control and a full-width "Convidar" button.
- **Categorias:** Gastos | Entradas; "Nova categoria" opens a sheet with a preview (icon and name), type segmented control, **color in a row of swatches** and **icon in an emoji grid with tabs by subject** (Casa, Comida, Transporte, Lazer, Saúde, Dinheiro and others). For expenses, the type (Essencial | Supérfluo | Investimento) is a segmented control.
- **Contas bancárias:** account, payment method, import bank statement (OFX or CSV, with column mapping only for CSV) and the rules.
- **Aparência:** Automático | Claro | Escuro.

**Login and sign-up.** Large fields, one button, the mode toggle as text. Sign-up keeps "Como você gostaria de ser chamado?" (What would you like to be called?) and the password confirmation (PR #26).

## Design system

**Colors** (light): background `#F2F3F4`, surface `#FFFFFF`, ink `#14181B`, secondary text `#7B858B`, line `#ECEEEF`, accent `#0E6577` (only action and active item), income `#1E8E5A`, alert `#C2412D` (budget overruns and destructive actions). Expenses are not red: money going out uses the normal ink. Dark uses the same roles with background `#0F1113`, surface `#191C1F` and accent `#6CC4D6`, following `prefers-color-scheme`, with the Aparência selector to force it.

**Typography:** one family, Geist (400, 500, 600, 700), tabular numbers. Large balance 44 px (mobile) and 54 px (desktop) at weight 700 with tight tracking; page titles 32 px; body 14 to 15 px; secondary text 12 to 13 px. Geist is served from the app itself (local file), without asking third parties.

**Shape:** grouped blocks with radius 22, controls with radius 12, buttons and selectors as pills; no heavy borders and no shadows, except the floating bar. Generous space: 20 px gutter on mobile, 56 px on desktop.

**Alignment:** text on the left, values on the right, nothing centered except empty states. This is a system rule, not decided screen by screen.

**Components:** `Sheet` (sheet with a handle, native `<dialog>`), `Segmented`, `GroupedList` and `ListRow` (with arrow, value and variants), `Switch`, `CategoryGrid`, `EmojiPicker`, `ColorSwatches`, `AmountInput`, `TabBar`, `Sidebar`, `Button` (primary, secondary, destructive), `EmptyState`. The tokens live in a single CSS file, with variables for light and dark.

**Motion:** only what responds to an action: sheet going up and down, switch, tab change. Respects `prefers-reduced-motion`.

**Accessibility:** visible focus, touch targets of at least 44 px, AA contrast in both themes, each list row as a single control with a clear name, sheet with trapped focus and Esc to close.

## Delivery

Five small PRs, each reviewable, with green CI before the merge. Each PR rewrites the tests of the screens it touches; the API and the data do not change.

1. **Base:** tokens, font and the new navigation (floating bar, sidebar, avatar for Ajustes). The screen components (sheet, segmented control, grouped list, pickers) arrive together with the first screen that uses them. The old screens keep working inside the new shell.
2. **Visão and New transaction** (sheet, visual pickers).
3. **Lançamentos** (with the Repetem todo mês section) and **Caixa**.
4. **All of Ajustes:** Pessoas, Categorias (color and emojis), Formas de pagamento (Payment methods), Orçamentos (Budgets), Contas bancárias, Aparência.
5. **Divisão, login and finishing touches:** empty states, errors, accessibility, a final alignment pass.

Each PR is merged into master only with the user's approval, and the merge triggers the deploy.

## Tests

- **Components and screens:** Vitest and Testing Library, by role and accessible name (already the case today). The new components have their own tests (segmented control, emoji picker, sheet: open, close, focus).
- **e2e (Playwright, desktop and Pixel 5):** the existing flows continue, with the new labels; they gain a "create category with color and emoji" flow, an "invite someone" flow and a "navigate using only the four tabs and the avatar" flow.
- **Visual regression:** screenshots of the main screens in light and dark, attached as a CI artifact, with no automatic comparison for now (comparison with tolerance comes later).
- **Lint and types** remain mandatory.

## Risks and decisions

- **Large scope.** Mitigated by the five PRs: the app stays usable at every merge.
- **Emojis depend on the system.** The grid uses operating system emojis, so the look varies between Windows, Android and iOS. That is acceptable; drawn icons are out of scope.
- **Recurrences out of the menu.** Whoever uses that screen a lot needs two more taps; in exchange, the navigation stays at four destinations. Revisit if it becomes annoying.
- **Conditional Divisão.** In wallets without splitting, the bar has three tabs plus the "+".
- **Local font.** Geist goes into the repository (font file); confirm the license (OFL) when adding it.
