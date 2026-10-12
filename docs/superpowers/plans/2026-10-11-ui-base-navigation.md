# Reestruturação da interface, PR 1: base e navegação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Trocar a base visual (tokens, fonte) e a navegação (barra flutuante no celular, barra lateral no desktop, Ajustes só pelo avatar) sem quebrar nenhuma tela existente.

**Architecture:** Os tokens atuais (`--paper`, `--surface`, `--ink`, `--muted`, `--line`, `--accent`...) mantêm os nomes e ganham os valores novos, então as telas antigas herdam o visual novo sem reescrita. A navegação sai do `Shell.tsx` para dois componentes (`TabBar`, `Sidebar`) que compartilham a lista de destinos em `nav.ts`. Os componentes de tela (`Segmented`, `ListRow`, `Sheet` etc.) chegam no PR que os usa primeiro, não aqui.

**Tech Stack:** React 19, React Router 8, Vite, Vitest + Testing Library, Playwright (desktop e Pixel 5), `@fontsource-variable/geist`.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), seções Navegação, Sistema de design e Entrega (PR 1).

## Global Constraints

- Código, comentários e testes em inglês; texto da interface em pt-BR.
- Cores claras: fundo `#F2F3F4`, superfície `#FFFFFF`, tinta `#14181B`, secundário `#7B858B`, linha `#ECEEEF`, acento `#0E6577`, entrada `#1E8E5A`, alerta `#C2412D`.
- Cores escuras: fundo `#0F1113`, superfície `#191C1F`, tinta `#F3F4F5`, secundário `#8A949A`, linha `#23272A`, acento `#6CC4D6`, entrada `#5BD08D`, alerta `#FF8A78`. Seguem `prefers-color-scheme`; `data-theme="light|dark"` força.
- Fonte única: Geist (variável), servida pelo próprio app via `@fontsource-variable/geist`, sem pedir a terceiros. Números tabulares.
- Raios: grupos 22 px, controles 12 px, botões e seletores em pílula. Sombra só na barra flutuante.
- Destinos: Visão `/`, Lançamentos `/lancamentos`, Caixa `/caixa`, Divisão `/divisao` (só quando a carteira divide gastos), mais "+" que abre Novo lançamento (só para quem pode editar).
- Ajustes: um único caminho, o avatar (`/ajustes`). Recorrentes sai da navegação.
- Barra lateral a partir de 900 px; abaixo, barra flutuante. Alvos de toque de pelo menos 44 px, foco visível, `aria-current="page"` no destino ativo, respeitar `env(safe-area-inset-bottom)`.
- A API, os dados e as telas internas não mudam neste PR.

## Review Focus

- Carteira sem divisão: a aba Divisão não existe (e a barra tem três abas mais o "+"). Testado na Task 2 e na Task 3.
- Leitor (`canEdit` falso): nenhum "+" nem "Novo lançamento", nem na barra nem na lateral. Task 2 e 3.
- Contador da Caixa: sumido em 0, número exato até 99, "99+" acima disso. Task 2.
- Recorrentes continua alcançável depois de sair do menu (um link em Lançamentos). Task 4.
- Modo escuro completo: todo papel de cor definido nos dois temas, incluindo `--tick` e `--tick-future` que a régua da Visão usa. Task 1.
- Só um caminho visível para Ajustes por viewport, e a carteira ativa continua trocável no desktop depois de esconder a `PageBar` lá. Task 3 e 5.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `web/src/styles/tokens.css` (novo) | Variáveis de cor, fonte e raio, claro e escuro |
| `web/src/styles.css` (modifica) | Importa os tokens, perde o bloco `:root` antigo, ganha o CSS de `.tabbar`, `.sidebar`; remove `.bottom`, `.fab` antigos |
| `web/src/main.tsx` (modifica) | Importa Geist no lugar de Bricolage e Instrument Sans |
| `web/src/components/Icon.tsx` (novo) | Ícones de traço: `home`, `list`, `tray`, `split`, `plus` |
| `web/src/components/nav.ts` (novo) | Lista de destinos e `formatBadge` |
| `web/src/components/TabBar.tsx` (novo) | Barra flutuante do celular |
| `web/src/components/Sidebar.tsx` (novo) | Barra lateral do desktop, com carteira e perfil |
| `web/src/Shell.tsx` (modifica) | Usa `TabBar` e `Sidebar`; sem lista de navegação própria |
| `web/src/pages/Transactions.tsx` (modifica) | Link "Repetem todo mês" para `/recorrentes` |

---

### Task 1: Tokens e fonte

**Files:**
- Create: `web/src/styles/tokens.css`
- Create: `web/src/styles/tokens.test.ts`
- Modify: `web/src/styles.css` (remove `:root` e os dois blocos escuros das linhas 1 a 14, adiciona `@import "./styles/tokens.css";` na primeira linha)
- Modify: `web/src/main.tsx`, `web/package.json`

**Interfaces:**
- Produces: variáveis CSS `--paper --surface --ink --muted --line --accent --on-accent --income --over --scrim --tick --tick-future --font --ui --display --radius-group --radius-control`. `--ui` e `--display` apontam ambos para `--font` (os nomes ficam para não tocar em cada regra; `--display` é removido no PR 5).

- [ ] **Step 1: Escrever o teste do contrato de tokens** em `tokens.test.ts`: lê `tokens.css` como texto e, para cada um dos três contextos (`:root`, `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"])`, `:root[data-theme="dark"]`), confirma que o bloco define `--paper`, `--surface`, `--ink`, `--muted`, `--line`, `--accent`, `--on-accent`, `--income`, `--over`, `--scrim`, `--tick`, `--tick-future`. Confirma também os valores exatos das Global Constraints para `--paper`, `--accent` e `--over` nos dois temas, e que o bloco escuro e o `[data-theme="dark"]` têm valores idênticos.
- [ ] **Step 2: Rodar `npx vitest run src/styles` e ver falhar** (arquivo inexistente).
- [ ] **Step 3: Criar `tokens.css`** com os valores das Global Constraints. `--tick`: `#B9C0C4` (claro) e `#5A646A` (escuro); `--tick-future`: `#D9DDE0` e `#2A2F33`; `--scrim`: `rgba(20,24,27,.28)` e `rgba(0,0,0,.55)`; `--font: "Geist Variable", system-ui, sans-serif`; `--radius-group: 22px`; `--radius-control: 12px`; `color-scheme` por tema.
- [ ] **Step 4: Trocar a fonte.** `npm remove @fontsource-variable/bricolage-grotesque @fontsource-variable/instrument-sans && npm i @fontsource-variable/geist`; em `main.tsx`, importar `@fontsource-variable/geist` no lugar dos dois antigos. Em `styles.css`, `body` usa `font: 400 15px/1.45 var(--ui)` (já usa) e `.balance`, `.auth h1`, `.settle .big`, `.amount input` seguem com `var(--display)`.
- [ ] **Step 5: Rodar tudo** (`npx vitest run`, `npm run lint`, `npx tsc --noEmit`, `npm run build`). Esperado: tudo verde, e o `dist` carrega um único arquivo de fonte Geist.
- [ ] **Step 6: Commit** `feat(web): new design tokens and the Geist typeface`.

---

### Task 2: Ícones, destinos e barra flutuante

**Files:**
- Create: `web/src/components/Icon.tsx`, `web/src/components/nav.ts`, `web/src/components/TabBar.tsx`
- Test: `web/src/components/TabBar.test.tsx`, `web/src/components/nav.test.ts`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces:
  - `type IconName = "home" | "list" | "tray" | "split" | "plus"`; `function Icon({ name }: { name: IconName }): JSX.Element` (SVG 24x24, traço 1,7, `aria-hidden`).
  - `type NavItem = { to: string; label: string; icon: IconName; end?: boolean }`; `function navItems(splitting: boolean): NavItem[]` retorna Visão, Lançamentos, Caixa e, só se `splitting`, Divisão, nessa ordem.
  - `function formatBadge(n: number): string` retorna `""` para `n <= 0`, `String(n)` até 99 e `"99+"` acima.
  - `function TabBar({ pending, onAdd }: { pending: number; onAdd: () => void }): JSX.Element`; lê `wallet.splittingEnabled` e `canEdit` de `useApp()`.

- [ ] **Step 1: Testes de `nav.test.ts`:** `navItems(false)` tem exatamente os rótulos `["Visão", "Lançamentos", "Caixa"]`; `navItems(true)` acrescenta `"Divisão"` por último; nenhum item aponta para `/ajustes` ou `/recorrentes`; `formatBadge` de `0`, `-1` → `""`, `4` → `"4"`, `99` → `"99"`, `100` e `250` → `"99+"`.
- [ ] **Step 2: Testes de `TabBar.test.tsx`** (com `renderApp`): a `nav` "Navegação" tem os links na ordem, com `href` corretos; carteira sem divisão não mostra "Divisão" e com divisão mostra; botão "Novo lançamento" chama `onAdd` ao clique e some quando `canEdit` é falso; com `pending=4` o link Caixa contém "4", com `0` não contém número, com `250` mostra "99+"; em `route="/lancamentos"` só Lançamentos tem `aria-current="page"`.
- [ ] **Step 3: Rodar e ver falhar.**
- [ ] **Step 4: Implementar** `Icon.tsx` (um `switch` por nome, caminhos do mockup aprovado), `nav.ts` e `TabBar.tsx` (`NavLink` com `end` na Visão; o contador é um `<b className="badge">` dentro do link da Caixa, sem `aria-label`).
- [ ] **Step 5: CSS em `styles.css`:** `.tabbar` fixo a 16 px do fundo (mais `env(safe-area-inset-bottom)`), com a pílula `.tabbar-pill` (altura 58, fundo translúcido com `backdrop-filter: blur(14px)`, única sombra do sistema) e o `.fab` redondo de 58 px ao lado, em `--accent`. Item ativo em `--accent`, peso 600. Remove as regras `.bottom` e o `.fab` antigo. Esconde `.tabbar` a partir de 900 px.
- [ ] **Step 6: Rodar os testes novos e ver passar.**
- [ ] **Step 7: Commit** `feat(web): floating tab bar, icons and shared nav items`.

---

### Task 3: Barra lateral e Shell

**Files:**
- Create: `web/src/components/Sidebar.tsx`, `web/src/components/Sidebar.test.tsx`
- Modify: `web/src/Shell.tsx`, `web/src/Shell.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `navItems`, `formatBadge`, `Icon` (Task 2).
- Produces: `function Sidebar({ pending, onAdd }: { pending: number; onAdd: () => void }): JSX.Element`.

- [ ] **Step 1: Testes de `Sidebar.test.tsx`:** mesma bateria da `TabBar` (ordem dos links, Divisão condicional, contador, `aria-current`, "Novo lançamento" que chama `onAdd` e some para leitor); mais: não há links "Recorrentes" nem "Ajustes" entre os destinos; existe um link de perfil com o nome do usuário, `href="/ajustes"` e nome acessível "Ajustes"; o seletor `Carteira` mostra a carteira ativa e, ao mudar, chama `setWalletId` com o id; mostra "com Bia" quando há outra pessoa.
- [ ] **Step 2: Atualizar `Shell.test.tsx`:** onde hoje procura "Recorrentes" ou "Ajustes" na navegação, passar a esperar os quatro destinos; um teste novo confirma que com `splittingEnabled: false` o `Shell` não renderiza "Divisão" em nenhuma das duas barras.
- [ ] **Step 3: Rodar e ver falhar.**
- [ ] **Step 4: Implementar `Sidebar.tsx`** (um `<aside className="sidebar" aria-label="Navegação">`: seletor de carteira no topo, destinos, espaço, botão "Novo lançamento", perfil em `<Link to="/ajustes" aria-label="Ajustes">`). **Em `Shell.tsx`:** apagar `NAV` e o JSX de `aside` e `nav.bottom`, renderizar `<Sidebar pending onAdd />` e `<TabBar pending onAdd />`; manter o atalho `N`.
- [ ] **Step 5: CSS:** `.sidebar` fixo de 248 px a partir de 900 px, fundo ligeiramente mais escuro que `--paper`, item ativo em bloco branco (`--surface`) com `--accent`; `.main` ganha o espaço lateral. Esconde `.bar` (a `PageBar`) a partir de 900 px, porque a carteira e o perfil passam a viver na barra lateral. Padding inferior de `.main` no celular deixa o conteúdo livre da barra flutuante.
- [ ] **Step 6: Rodar `npx vitest run` e ver tudo passar.**
- [ ] **Step 7: Commit** `feat(web): sidebar and the new shell navigation`.

---

### Task 4: Recorrentes continua alcançável

**Files:**
- Modify: `web/src/pages/Transactions.tsx`, `web/src/pages/Transactions.test.tsx`

**Interfaces:**
- Consumes: rota `/recorrentes` (existe, não muda).

- [ ] **Step 1: Teste em `Transactions.test.tsx`:** a página mostra um link "Repetem todo mês" com `href="/recorrentes"`, para quem pode editar e para leitores.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** o link logo abaixo do título, como texto de acento com seta (`›`), provisório até o PR 3 mover a seção para dentro da lista.
- [ ] **Step 4: Rodar e ver passar.**
- [ ] **Step 5: Commit** `feat(web): keep recurrences reachable from Lançamentos`.

---

### Task 5: e2e, captura e fechamento

**Files:**
- Modify: `web/e2e/app.spec.ts`
- Modify: `docs/superpowers/specs/2026-10-11-ui-redesign-design.md` (Entrega, item 1)

- [ ] **Step 1: Ajustar os e2e existentes:** onde chegam em Recorrentes pelo menu (`getByRole("link", { name: /Recorrentes/ })`), ir por Lançamentos e "Repetem todo mês". Onde clicam em "Ajustes", continuam por `getByRole("link", { name: "Ajustes" }).first()`.
- [ ] **Step 2: Novo teste e2e "navigation":** nos dois projetos, depois de cadastrar, a navegação tem Visão, Lançamentos e Caixa visíveis; Divisão não existe numa carteira sem divisão; exatamente um link "Ajustes" visível; "Novo lançamento" abre a folha; no desktop a carteira ativa aparece na barra lateral.
- [ ] **Step 3: Rodar a suíte** (`npm run build && npm run e2e`) em desktop e mobile. Esperado: todos passam.
- [ ] **Step 4: Atualizar a spec:** na seção Entrega, o item 1 passa a dizer "tokens, fonte e navegação nova; os componentes de tela chegam junto da primeira tela que os usa".
- [ ] **Step 5: Abrir o PR** com capturas de tela do celular (claro e escuro) e do desktop na descrição. Merge só com o aval do usuário (dispara o deploy).
- [ ] **Step 6: Commit** `test(e2e): navigation, and the spec note on component timing`.
