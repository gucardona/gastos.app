# Reestruturação da interface, PR 2: Visão e Novo lançamento

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dar à Visão e à folha de lançamento (novo e edição) a linguagem visual nova, com os componentes de tela que as duas telas precisam e que as próximas reaproveitam.

**Architecture:** Cinco componentes novos e pequenos (`Segmented`, `ListGroup` com `ListRow`, `Sheet`, e as escolhas visuais `CategoryGrid`, `OptionList`, `DatePicker` dentro de uma `PickerView`) substituem o CSS e o markup soltos que hoje vivem em `AddSheet.tsx` e `Overview.tsx`. A folha passa a ser uma lista agrupada: tocar em Categoria, Quando ou Pago com troca a folha por uma escolha visual (sem diálogos aninhados) e um botão Voltar traz de volta. A Visão mantém todo o conteúdo e os testes atuais, reorganizados em blocos agrupados.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop e Pixel 5), CSS puro com os tokens do PR 1.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), seções Telas (Visão, Novo lançamento), Sistema de design e Entrega (PR 2). Base: PR 1 já no master (tokens, Geist, navegação).

## Global Constraints

- Código, comentários e testes em inglês; texto da interface em pt-BR.
- Blocos agrupados com raio `var(--radius-group)` (22 px) e fundo `var(--surface)`; controles com `var(--radius-control)` (12 px); botões e seletores em pílula; sem bordas pesadas e **sem sombras** (a única é a da barra flutuante).
- Texto à esquerda, valores à direita; centralizado só em estados vazios e no valor da folha.
- Saldo da Visão: 44 px (celular) e 54 px (desktop), peso 700, tracking apertado, números tabulares.
- Alvos de toque de pelo menos 44 px; foco visível; contraste AA nos dois temas (os tokens já passam, não criar cores novas fora de `tokens.css`).
- A folha continua sendo um `<dialog>` nativo (foco preso, Esc fecha); lateral de 440 px no desktop, de baixo para cima no celular.
- Gasto não é vermelho: usa a tinta normal. Entrada em `--income`. Alerta (`--over`) só para estouro de orçamento e ações destrutivas.
- Movimento só em resposta a uma ação (folha subindo, interruptor), com `prefers-reduced-motion` respeitado.
- A API, os dados e as regras de negócio não mudam; todos os comportamentos hoje cobertos por teste em `AddSheet.test.tsx` e `Overview.test.tsx` continuam cobertos (mesmos nomes de comportamento, seletores atualizados).

## Review Focus

- Foco e teclado da folha: Esc fecha a folha inteira (mesmo dentro de uma escolha); o botão Voltar de uma escolha devolve o foco à linha que a abriu. Task 3 e 5.
- Valor digitado em formatos reais: `48,90`, `1.234,56`, `0`, vazio e texto inválido; só valores maiores que zero salvam. Task 5.
- Editar um lançamento cuja categoria está arquivada: a categoria aparece e continua selecionada, e as outras arquivadas não são oferecidas. Task 4 e 5.
- Muitas categorias (20 ou mais): a grade rola dentro da folha, a escolhida fica marcada e o botão Salvar continua visível. Task 4.
- Carteira que divide gastos: "Quem pagou" e "Dividir" continuam funcionando, e porcentagens que não somam 100% são recusadas antes do envio. Task 5.
- Descrição longa (acima de 200 caracteres): a mensagem de erro do servidor aparece sob o campo, sem fechar a folha. Task 5.
- Mês sem lançamentos e saldo negativo na Visão: estado vazio claro e saldo destacado sem usar vermelho de gasto. Task 6.
- Leitor: sem "+" e sem abrir a folha ao tocar numa linha da lista. Task 6.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `web/src/components/Segmented.tsx` (novo) | Controle segmentado genérico (substitui `.seg`) |
| `web/src/components/ListGroup.tsx` (novo) | Bloco agrupado e linha de lista (`ListGroup`, `ListRow`) |
| `web/src/components/Switch.tsx` (modifica) | Interruptor no estilo novo |
| `web/src/components/Sheet.tsx` (novo) | Folha modal sobre `<dialog>` |
| `web/src/components/pickers.tsx` (novo) | `PickerView`, `CategoryGrid`, `OptionList`, `DatePicker` |
| `web/src/components/AddSheet.tsx` (modifica) | Folha de lançamento em lista agrupada com escolhas visuais |
| `web/src/components/PageBar.tsx` (modifica) | `MonthSwitcher` vira o título grande do mês |
| `web/src/pages/Overview.tsx` (modifica) | Saldo, entrou e saiu, "Para onde foi" e "Recentes" em blocos |
| `web/src/components/TransactionList.tsx` (modifica) | Linhas dentro de blocos agrupados |
| `web/src/styles.css` (modifica) | Estilos dos componentes novos; remove `.seg`, `.chips` e regras antigas da folha |

---

### Task 1: Controle segmentado

**Files:**
- Create: `web/src/components/Segmented.tsx`, `web/src/components/Segmented.test.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces: `function Segmented<T extends string>(props: { label: string; value: T; onChange: (v: T) => void; options: { value: T; label: string }[] }): JSX.Element`. Renderiza `<div role="group" aria-label={label} className="segmented">` com um `<button type="button" aria-pressed>` por opção.

- [ ] **Step 1: Testes em `Segmented.test.tsx`:** o grupo tem o nome dado; cada opção é um botão com `aria-pressed` verdadeiro só para a atual; clicar numa opção chama `onChange` com o valor dela (e clicar na já ativa não quebra); as opções aparecem na ordem recebida; ←/→ não são exigidos (os botões são tabuláveis).
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** `Segmented` e o CSS `.segmented` (fundo `color-mix(in srgb, var(--ink) 8%, transparent)`, raio 12, padding 3, opção ativa em `--surface` e peso 600, altura mínima 36 px).
- [ ] **Step 4: Rodar e ver passar.**
- [ ] **Step 5: Commit** `feat(web): segmented control`.

---

### Task 2: Lista agrupada e interruptor

**Files:**
- Create: `web/src/components/ListGroup.tsx`, `web/src/components/ListGroup.test.tsx`
- Modify: `web/src/components/Switch.tsx`, `web/src/styles.css`

**Interfaces:**
- Produces:
  - `function ListGroup(props: { children: ReactNode; className?: string }): JSX.Element`: bloco branco de raio 22.
  - `function ListRow(props: { label: ReactNode; value?: ReactNode; icon?: ReactNode; onClick?: () => void; chevron?: boolean; trailing?: ReactNode; disabled?: boolean }): JSX.Element`: com `onClick` renderiza `<button type="button" className="list-row">`, sem ele um `<div className="list-row">`. Ordem visual: ícone, rótulo, valor (à direita, `--muted`), `trailing`, seta (`›`, `aria-hidden`) quando `chevron`.
  - `Switch` mantém a assinatura `{ checked, onChange, label, disabled }`; só muda o visual.

- [ ] **Step 1: Testes em `ListGroup.test.tsx`:** uma linha com `onClick` é um botão cujo nome acessível junta rótulo e valor ("Categoria Mercado") e dispara o clique; sem `onClick` não é botão; `chevron` não entra no nome acessível; `disabled` desativa o botão; `trailing` renderiza o que for passado (um `Switch` funciona dentro de uma linha sem `onClick`, e alternar chama `onChange`).
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** os dois componentes e o CSS (`.group` com padding vertical 4 e horizontal 16, `.list-row` com altura mínima 48, divisor de 1 px em `--line` entre linhas, sem divisor na primeira; `Switch` de 51x31 com o botão branco e fundo `--accent` quando ligado).
- [ ] **Step 4: Rodar e ver passar; rodar a suíte inteira** (o `Switch` já é usado em Recorrentes e Ajustes).
- [ ] **Step 5: Commit** `feat(web): grouped list rows and the iOS-style switch`.

---

### Task 3: Folha

**Files:**
- Create: `web/src/components/Sheet.tsx`, `web/src/components/Sheet.test.tsx`
- Modify: `web/src/components/AddSheet.tsx` (só para usar `Sheet`), `web/src/styles.css`

**Interfaces:**
- Produces: `function Sheet(props: { open: boolean; onClose: () => void; label: string; children: ReactNode }): JSX.Element`. `<dialog className="sheet" aria-label={label}>` modal; abre com `showModal()` quando `open` fica verdadeiro e fecha com `close()` quando falso; `onClose` é chamado em Esc e em clique no fundo; os filhos só são renderizados enquanto `open`. Inclui o puxador (`.grab`), escondido no desktop.

- [ ] **Step 1: Testes em `Sheet.test.tsx`:** fechada não renderiza os filhos; `open` renderiza e o diálogo tem o nome dado; Esc chama `onClose`; clique no fundo chama `onClose`, clique dentro não; ao passar `open` de verdadeiro para falso o diálogo fecha. (jsdom: usar `HTMLDialogElement.prototype.showModal/close` simulados em `src/test/setup.ts` se ainda não existirem.)
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar `Sheet`** extraindo o que `AddSheet` já faz com o `<dialog>` (efeito de abrir e fechar, clique no fundo). `AddSheet` passa a renderizar `<Sheet open={state !== null} onClose={onClose} label={...}>` com o mesmo conteúdo de hoje, sem mudar comportamento.
- [ ] **Step 4: CSS:** fundo `--surface`, topo arredondado 28 px, `::backdrop` com `--scrim`, animação de subida que respeita `prefers-reduced-motion`; a partir de 900 px painel lateral de 440 px.
- [ ] **Step 5: Rodar a suíte inteira** (`AddSheet.test.tsx` segue verde, é a prova de que nada mudou).
- [ ] **Step 6: Commit** `feat(web): Sheet component extracted from the quick-add dialog`.

---

### Task 4: Escolhas visuais

**Files:**
- Create: `web/src/components/pickers.tsx`, `web/src/components/pickers.test.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces:
  - `function PickerView(props: { title: string; onBack: () => void; children: ReactNode }): JSX.Element`: cabeçalho com botão "Voltar" (nome acessível `Voltar`) e o título (`<h2>`), e os filhos abaixo.
  - `function CategoryGrid(props: { categories: Category[]; value: number | null; onPick: (id: number) => void }): JSX.Element`: grade de 3 colunas (celular) com um botão por categoria (emoji e nome, `aria-pressed`); a grade rola quando é alta; marca a escolhida.
  - `function OptionList<T extends string | number>(props: { options: { value: T; label: string }[]; value: T | null; onPick: (v: T) => void }): JSX.Element`: lista agrupada de linhas com `aria-pressed` e um visto na escolhida.
  - `function DatePicker(props: { value: string; onPick: (iso: string) => void }): JSX.Element`: linhas "Hoje" e "Ontem" (marcadas se coincidirem com `value`) e um campo de data nativo com `aria-label="Data do lançamento"` para outro dia.

- [ ] **Step 1: Testes em `pickers.test.tsx`:** `PickerView` mostra o título e `Voltar` chama `onBack`; `CategoryGrid` mostra todas as categorias recebidas (inclusive 25), marca só a de `value`, `onPick` recebe o id, e uma lista vazia não quebra; `OptionList` marca a escolhida e chama `onPick` com o valor tipado (número ou texto); `DatePicker` marca Hoje quando `value` é hoje, Ontem quando é ontem, nenhum dos dois para outra data (e mostra o campo de data com esse valor), e escolher Ontem chama `onPick` com a data de ontem em `YYYY-MM-DD`.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** os quatro componentes reutilizando `ListGroup` e `ListRow` da Task 2 e `today()` de `lib/dates.ts`; extrair para `lib/dates.ts` um `yesterday(): string` (hoje duplicado dentro de `AddSheet.tsx`) com teste em `dates.test.ts`.
- [ ] **Step 4: CSS** da grade (`.category-grid`, `.cat-tile` com raio 16, emoji de 26 px, nome em 12 px, selecionada com anel `--accent`, altura máxima de 46vh com rolagem).
- [ ] **Step 5: Rodar e ver passar.**
- [ ] **Step 6: Commit** `feat(web): visual pickers for category, option and date`.

---

### Task 5: Folha de lançamento em lista agrupada

**Files:**
- Modify: `web/src/components/AddSheet.tsx`, `web/src/components/AddSheet.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Sheet`, `Segmented`, `ListGroup`, `ListRow`, `Switch`, `PickerView`, `CategoryGrid`, `OptionList`, `DatePicker`, `yesterday`.
- Produces: o mesmo `AddSheet({ state, onClose })`; nenhum chamador muda.

Layout da folha (de cima para baixo): puxador; `Segmented` "Tipo" (Gasto | Entrada) com o botão Fechar (`aria-label="Fechar"`) ao lado; valor grande centralizado (`input` com `aria-label="Valor"`, `inputMode="decimal"`, prefixo "R$"); um `ListGroup` com as linhas **Categoria** (valor "🛒 Mercado"), **Quando** (valor "Hoje", "Ontem" ou a data por extenso), **Pago com** (nome da forma, só se existir alguma), **Descrição** (campo de texto na própria linha, `aria-label="Descrição"`) e, em criação, **Repetir todo mês** (`Switch`); em carteira que divide gastos entram **Quem pagou** (escolha) e **Dividir** (resumo e botão Ajustar/Igualar, com os campos de porcentagem abaixo como hoje); por fim o botão "Salvar" de largura total, "Salvar e lançar outro" (só criação) e, na edição, "Excluir lançamento" com confirmação.

- [ ] **Step 1: Reescrever `AddSheet.test.tsx`** mantendo um teste por comportamento hoje coberto (os 14 `it` existentes: formulário vazio não envia e mostra o que falta; cria o gasto com a forma de pagamento padrão e fecha; troca para entrada e usa as categorias de entrada; "Salvar e lançar outro" mantém a folha e limpa valor e descrição; erro de campo da API; mensagem geral quando a API falha; "Repetir todo mês" cria recorrência na data; sem divisão em carteira sem divisão; porcentagens que não somam 100% recusadas e enviadas quando somam; divisão padrão não envia `splits`; escolher quem pagou; edição pré-preenchida e salva com PUT; exclusão só após confirmar; sem "Repetir" nem "lançar outro" na edição), trocando os seletores de chip por: abrir a linha **Categoria**, tocar na categoria na grade e voltar. Acrescentar:
  - abrir **Categoria** mostra `PickerView` "Categoria" e `Voltar` leva de volta à folha com o valor da linha atualizado;
  - escolher **Ontem** em Quando envia a data de ontem; **Outro dia** com campo de data envia a data digitada;
  - valores: `48,90` envia 4890, `1.234,56` envia 123456, `0` e `abc` não enviam e mostram "Informe um valor maior que zero.";
  - editar com categoria arquivada: a linha Categoria mostra a categoria arquivada e a grade só oferece ela e as ativas;
  - com 25 categorias a grade lista as 25;
  - `Voltar` devolve o foco à linha que abriu a escolha;
  - descrição acima de 200 caracteres: o erro devolvido pela API aparece e a folha não fecha.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Reescrever `AddForm`** com a estrutura acima. Estado novo: `picker: null | "category" | "date" | "method" | "paidBy"`; quando não é `null` a folha mostra a `PickerView` correspondente no lugar do formulário (o estado do formulário não se perde); escolher fecha a escolha e volta. Toda a lógica de salvar, validar, excluir e dividir permanece como está.
- [ ] **Step 4: CSS:** valor grande (`font-size: 52px`, peso 700, tracking -0.04em, centralizado, prefixo "R$" menor e `--muted`), linhas da folha com os componentes da Task 2, remover `.seg`, `.chips` e `.field` que ficarem sem uso (conferir com `grep` antes de apagar).
- [ ] **Step 5: Rodar `AddSheet.test.tsx` e a suíte inteira.** Esperado: tudo verde.
- [ ] **Step 6: Commit** `feat(web): quick-add sheet as a grouped list with visual pickers`.

---

### Task 6: Visão

**Files:**
- Modify: `web/src/pages/Overview.tsx`, `web/src/pages/Overview.test.tsx`, `web/src/components/PageBar.tsx`, `web/src/components/PageBar.test.tsx`, `web/src/components/TransactionList.tsx`, `web/src/components/TransactionList.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `ListGroup`, `ListRow` (Task 2).
- Produces: `MonthSwitcher()` sem mudar assinatura nem rótulos acessíveis ("Mês anterior", "Próximo mês"); o mês passa a ser um `<h1 className="month-title">` grande (o texto continua `monthLabel(month)`, por exemplo "janeiro de 2026", com a primeira letra maiúscula só por CSS).

- [ ] **Step 1: Testes primeiro.** `PageBar.test.tsx`: o mês aparece como título de nível 1 e a navegação para os vizinhos segue igual. `Overview.test.tsx`: manter todos os comportamentos de hoje (separação de reais e centavos, saldo negativo destacado, média diária e mês anterior, estados vazio, carregando e erro, "Ver todos", régua do mês, trilhos de orçamento com marcador e alerta, acerto sugerido) e acrescentar: "Para onde foi" é um bloco com uma linha por categoria de gasto (nome, valor e barra), "Recentes" lista os últimos 6 lançamentos e tocar numa linha abre a edição para quem edita e não faz nada para o leitor; saldo negativo usa a classe `negative` mas **não** a cor de gasto. `TransactionList.test.tsx`: as linhas continuam agrupadas por dia com o total do dia.
- [ ] **Step 2: Rodar e ver falhar** (os novos; os antigos que dependiam de markup mudado serão atualizados no Step 3).
- [ ] **Step 3: Implementar.** `MonthSwitcher` como título; hero com saldo grande, uma linha "Entrou" e "Saiu" com pontos de cor (entrada em `--income`, saída em tinta); "Para onde foi" em `ListGroup` com `ListRow` por categoria mantendo as classes `rail`, `track`, `fill`, `limit` e `over` que os testes e o marcador de orçamento usam; "Recentes" com `TransactionList` dentro de um `ListGroup`; o cartão de acerto no mesmo bloco, com "Registrar acerto" como ação primária. A régua dos dias continua, só reestilizada.
- [ ] **Step 4: CSS do desktop** (a partir de 900 px): `.hero` vira três blocos lado a lado (saldo, entrou, saiu) e as duas colunas "Para onde foi" e "Recentes"; no celular, uma coluna.
- [ ] **Step 5: Rodar `npx vitest run`, `tsc`, `npm run lint`, `npm run build`.** Esperado: tudo verde.
- [ ] **Step 6: Commit** `feat(web): overview in grouped blocks with the large month title`.

---

### Task 7: e2e, capturas e fechamento

**Files:**
- Modify: `web/e2e/app.spec.ts`

- [ ] **Step 1: Ajustar os e2e** que usam a folha antiga: o helper `addExpense` passa a abrir Categoria (grade), escolher e voltar, em vez de clicar num chip; o fluxo "Repetir todo mês" usa a linha com interruptor; os que editam e excluem seguem os mesmos rótulos ("Salvar", "Excluir lançamento", "Confirmar exclusão").
- [ ] **Step 2: Novo e2e "quick add":** nos dois projetos, abrir "Novo lançamento", digitar `48,90`, escolher Categoria e Quando (Ontem), salvar, e ver o lançamento em Lançamentos com a data de ontem; abrir de novo e confirmar que o Esc fecha a folha sem salvar.
- [ ] **Step 3: Rodar** `npm run build && npm run e2e`. Esperado: todos passam.
- [ ] **Step 4: Capturas** (celular e desktop, claro e escuro, Visão e folha) com um spec temporário que **não** é commitado; conferir à vista e corrigir o que destoar do mockup aprovado antes do commit.
- [ ] **Step 5: Abrir o PR** descrevendo o que mudou e que o merge dispara o deploy; o merge exige o aval do usuário e hoje o `--admin`.
- [ ] **Step 6: Commit** `test(e2e): quick add with visual pickers`.
