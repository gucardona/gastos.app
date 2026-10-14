# Reestruturação da interface, PR 3: Lançamentos e Caixa

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Levar a linguagem nova a Lançamentos e à Caixa, e mover Recorrentes para dentro de Lançamentos como a seção "Repetem todo mês", sem perder nenhum comportamento atual.

**Architecture:** Lançamentos ganha o filtro segmentado, a busca e os totais no estilo da Visão, e uma seção "Repetem todo mês" no fim da página (lista agrupada, interruptor para pausar, toque na linha abre uma folha de edição). A Caixa passa de formulário por linha a lista agrupada: tocar numa linha abre uma folha com a categoria (grade), a descrição, a regra opcional e as ações. As duas folhas reusam `Sheet`, `ListGroup`/`ListRow`, `Switch`, `Segmented` e `pickers` do PR 2. A página `/recorrentes` deixa de existir como destino e redireciona para Lançamentos.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop e Pixel 5), CSS puro com os tokens do PR 1.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), seções Telas (Lançamentos, Caixa), Navegação e Entrega (PR 3). Base: PRs 1 e 2 no master.

## Global Constraints

- Código, comentários e testes em inglês; texto da interface em pt-BR.
- Mesmo sistema do PR 2: blocos agrupados de raio 22, controles de raio 12, sem sombras, texto à esquerda e valores à direita, alvos de toque de pelo menos 44 px, foco visível, contraste AA nos dois temas, e só tokens de `tokens.css` (nada de cor nova).
- Folhas são `Sheet` (diálogo nativo); a escolha de categoria é um `PickerView` dentro da própria folha, como na folha de lançamento.
- Gasto não é vermelho; entrada em `--income`; `--over` só para alerta e ação destrutiva (Ignorar e Excluir são secundários, a exclusão pede confirmação).
- Recorrentes sai da navegação e da rota: `/recorrentes` passa a redirecionar para `/lancamentos`. A seção "Repetem todo mês" aparece para quem pode editar e para leitores (leitores veem sem controles).
- A API, os dados e as regras não mudam. Todo comportamento coberto hoje por `Transactions.test.tsx`, `Recurring.test.tsx` e `Inbox.test.tsx` continua coberto (mesmos comportamentos, seletores atualizados).

## Review Focus

- Pausar uma recorrência envia o objeto inteiro com `enabled=false` (e retomar, com `true`); recorrências do tipo acerto (sem categoria) aparecem e editam sem quebrar. Task 3.
- Leitor: sem interruptor e sem toque nas linhas de recorrência, e sem ações na Caixa. Task 3 e 4.
- Quem abre `/recorrentes` por favorito cai em Lançamentos, não numa página em branco. Task 3.
- Caixa: uma linha que outra pessoa acabou de tratar (409) mostra a mensagem do servidor e a lista se atualiza; uma linha de entrada só oferece categorias de entrada. Task 4.
- Regra de categoria: texto com menos de 2 caracteres é recusado antes de aceitar a linha (hoje a linha seria aceita e a regra falharia). Task 4.
- Caixa com muitas linhas (200 ou mais): a lista rola, "Aceitar N sugeridas" conta só as seguras. Task 4.
- Lançamentos: busca e filtro combinados, totais do mês inteiro, e mês sem lançamentos com as recorrências ainda visíveis. Task 1 e 3.
- Sem rolagem lateral entre 900 e 1100 px em Lançamentos e na Caixa. Task 5.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `web/src/pages/Transactions.tsx` (modifica) | Filtro segmentado, busca e totais no estilo novo; inclui `RecurringSection` |
| `web/src/components/RecurrenceSheet.tsx` (novo) | Folha de edição e exclusão de uma recorrência; exporta `recurrenceBody` |
| `web/src/components/RecurringSection.tsx` (novo) | Seção "Repetem todo mês" |
| `web/src/pages/Recurring.tsx`, `Recurring.test.tsx` (apaga) | Substituídos pelos dois arquivos acima |
| `web/src/App.tsx` (modifica) | `/recorrentes` redireciona para `/lancamentos` |
| `web/src/components/InboxSheet.tsx` (novo) | Folha de decisão de uma linha do extrato |
| `web/src/pages/Inbox.tsx` (modifica) | Lista agrupada que abre `InboxSheet` |
| `web/src/styles.css` (modifica) | Estilos novos; remove `.filters`, `.search` e `.item`/`.inline-form` que ficarem sem uso (conferir com `grep`; Ajustes ainda usa `.item` e `.inline-form` até o PR 4) |

---

### Task 1: Lançamentos no estilo novo

**Files:**
- Modify: `web/src/pages/Transactions.tsx`, `web/src/pages/Transactions.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Segmented` (PR 2).
- Produces: a página mantém o nome acessível dos controles: busca `Buscar` (`type="search"`), grupo `Filtros` com os botões `Tudo`, `Gastos`, `Entradas` (`aria-pressed`).

- [ ] **Step 1: Testes primeiro** em `Transactions.test.tsx`: manter todos os comportamentos atuais (totais do mês e lista por padrão, busca sem diferenciar maiúsculas, filtro de gastos e de entradas, filtro com busca com totais do mês inteiro, mês vazio convida a criar, leitor sem o convite, mês certo na API, agrupado por dia, erro com nova tentativa, carregando). Acrescentar: o filtro é um grupo `Filtros` com três botões e só `Tudo` marcado no início; os totais mostram "Entrou" e "Saiu" com a mesma marcação de pontos da Visão (`.flows`); o campo de busca tem altura mínima de 44 px (classe `search` com a regra no CSS, verificada no Step 4).
- [ ] **Step 2: Rodar e ver falhar** o que for novo.
- [ ] **Step 3: Implementar.** Trocar os chips por `<Segmented label="Filtros" ...>` com os três valores; os totais passam a usar o bloco `.flows` da Visão; a busca ganha o estilo de campo arredondado de raio 12 e fundo de superfície. Tirar o link provisório "Repetem todo mês ›" (a seção entra na Task 3).
- [ ] **Step 4: CSS:** `.search` com `min-height: 44px`; apagar `.filters` e `.chip` apenas se `grep` mostrar que nenhum outro componente os usa (Ajustes e Caixa ainda usam `.chip`).
- [ ] **Step 5: Rodar a suíte inteira.** Esperado: tudo verde.
- [ ] **Step 6: Commit** `feat(web): Lançamentos with the segmented filter and the new totals`.

---

### Task 2: Folha de recorrência

**Files:**
- Create: `web/src/components/RecurrenceSheet.tsx`, `web/src/components/RecurrenceSheet.test.tsx`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `useAction` (hook existente em `hooks/useAction.ts`).
- Produces:
  - `function recurrenceBody(r: Recurrence, over: Partial<Recurrence>, splitting: boolean): object`: o corpo completo do PUT (`kind, amountCents, description, categoryId, paymentMethodId, paidByUserId, toUserId, dayOfMonth, startDate, endDate, enabled`), com `splits` (apenas `userId` e `shareBp`) só quando `splitting` e o tipo é gasto e há partes. É a lógica que hoje vive dentro de `Recurring.tsx`.
  - `function RecurrenceSheet(props: { recurrence: Recurrence | null; onClose: () => void }): JSX.Element`: `Sheet` com o rótulo "Editar recorrência"; fechada quando `recurrence` é `null`.

Conteúdo da folha: valor (campo `Valor`), descrição (`Descrição`), dia do mês (`Dia do mês`), data final opcional (`Data final`), botão "Salvar", e "Excluir recorrência" com confirmação ("Confirmar exclusão"). Mensagens de validação como hoje: "Informe um valor maior que zero." e "O dia deve estar entre 1 e 31."

- [ ] **Step 1: Testes em `RecurrenceSheet.test.tsx`**, portando os comportamentos hoje cobertos em `Recurring.test.tsx`: edita valor e dia validando antes de enviar (valor zero e dia 0 ou 32 não enviam e mostram a mensagem), envia o objeto inteiro por PUT em `/api/wallets/1/recurrences/{id}` preservando os campos que a folha não mostra; só envia as partes da divisão quando a carteira divide gastos e o tipo é gasto; exclui só depois de confirmar (DELETE); fecha ao salvar e ao excluir; uma recorrência de acerto (`kind: "settlement"`, `categoryId: null`) abre e salva sem exigir categoria. Testes unitários de `recurrenceBody`: com `over` aplicado, sem `splits` quando a carteira não divide ou o tipo é entrada ou acerto, com `splits` mapeado quando divide.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** `recurrenceBody` e `RecurrenceSheet` (formulário dentro da folha, estado local, `useAction` para a chamada e o erro).
- [ ] **Step 4: Rodar e ver passar.**
- [ ] **Step 5: Commit** `feat(web): recurrence edit sheet`.

---

### Task 3: Seção "Repetem todo mês" e fim da página Recorrentes

**Files:**
- Create: `web/src/components/RecurringSection.tsx`, `web/src/components/RecurringSection.test.tsx`
- Modify: `web/src/pages/Transactions.tsx`, `web/src/pages/Transactions.test.tsx`, `web/src/App.tsx`, `web/src/styles.css`
- Delete: `web/src/pages/Recurring.tsx`, `web/src/pages/Recurring.test.tsx`

**Interfaces:**
- Consumes: `recurrenceBody`, `RecurrenceSheet` (Task 2), `ListGroup`, `ListRow`, `Switch`, `useRecurrences`, `useAction`.
- Produces: `function RecurringSection(): JSX.Element`: título `Repetem todo mês` (`<h2>`), um `ListGroup` com uma linha por recorrência (ícone da categoria ou ⇄ para acerto, título, "Todo dia N, próxima em D de mês" / ", pausada" / ", encerrada", valor com sinal, e um `Switch` com o nome `Pausar <título>` ou `Retomar <título>`); tocar na linha abre `RecurrenceSheet`; sem recorrências mostra a dica "Nenhuma recorrência ainda. Ao lançar um gasto ou uma entrada, ative Repetir todo mês." (para quem edita) ou nada (leitor).

- [ ] **Step 1: Testes em `RecurringSection.test.tsx`**, portando `Recurring.test.tsx`: lista com dia, próxima data e valor; estado vazio com a dica; pausar envia o objeto inteiro com `enabled=false` (e retomar com `true`); só envia partes da divisão quando a carteira divide; leitor vê a lista sem interruptor e sem abrir nada; tocar na linha abre a folha de edição da recorrência certa; recorrência de acerto mostra "Acerto de Ana para Bia". Em `Transactions.test.tsx`: a seção aparece na página (com a lista de lançamentos acima), inclusive quando o mês não tem lançamentos; troca o teste do link "Repetem todo mês" por um da seção. Teste de rota em `App.test.tsx` (novo, com `MemoryRouter` em `/recorrentes`): termina em Lançamentos.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** `RecurringSection` (linhas como `ListRow` sem `onClick` aninhando um botão de abrir e o `Switch`: a linha tem dois controles, então o conteúdo clicável é um botão `list-row` à esquerda e o `Switch` fica ao lado, fora do botão), incluí-la no fim de `Transactions`, trocar a rota por `<Route path="recorrentes" element={<Navigate to="/lancamentos" replace />} />`, apagar `Recurring.tsx` e `Recurring.test.tsx`, e remover os estilos que ficarem sem uso.
- [ ] **Step 4: Rodar a suíte inteira** (`npx vitest run`, `tsc`, `npm run lint`). Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): recurrences live inside Lançamentos`.

---

### Task 4: Caixa

**Files:**
- Create: `web/src/components/InboxSheet.tsx`, `web/src/components/InboxSheet.test.tsx`
- Modify: `web/src/pages/Inbox.tsx`, `web/src/pages/Inbox.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `Switch`, `PickerView`, `CategoryGrid`, `useAction`, `InboxItem` (tipos).
- Produces: `function InboxSheet(props: { item: InboxItem | null; onClose: () => void }): JSX.Element`. Conteúdo: valor e data (somente leitura), linha **Categoria** (abre a grade só com categorias do tipo da linha, a sugerida já marcada), campo **Descrição** (começa com o nome sugerido ou o texto do extrato), quando não há categoria sugerida uma linha **Usar nas próximas importações** com `Switch` e, ligado, o campo **Texto da regra** (começa com as duas primeiras palavras da descrição em minúsculas); ações **Aceitar** (desligada sem categoria), **É este lançamento** (só com candidato) e **Ignorar**.

A página `Inbox` mostra "Aceitar N sugeridas" no topo quando houver, e as linhas em um `ListGroup` (descrição ou nome sugerido, "data, conta" e valor com sinal; uma segunda linha pequena "Parece o lançamento que você já fez: …" quando há candidato); tocar abre `InboxSheet`; leitor vê as linhas sem abrir nada. Vazia: a mesma explicação de hoje com o link para `/ajustes?s=bancos`.

- [ ] **Step 1: Testes** portando os 11 comportamentos de `Inbox.test.tsx` (vazia aponta para os ajustes; aceita com a categoria escolhida e envia só `{ categoryId }` mais a descrição quando editada; não aceita sem categoria; só oferece categorias do tipo da linha; começa da categoria e do nome sugeridos; cria a regra depois de aceitar; liga ao lançamento parecido em vez de criar; ignora; aceita todas as sugeridas sem contar as que pedem decisão; mostra o erro do servidor; leitor só olha), agora pela folha: tocar na linha, escolher na grade, tocar em Aceitar. Acrescentar: regra com texto de menos de 2 caracteres é recusada **antes** de chamar a API ("Use ao menos 2 caracteres na regra."); uma linha tratada por outra pessoa (409) mostra "Esta linha já foi tratada" e a folha continua aberta; descrição editada é enviada; com 250 linhas a lista renderiza todas e o botão diz "Aceitar N sugeridas" com N só das seguras.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** `InboxSheet` (mesmo padrão de `AddSheet`: estado do formulário no componente, `picker` para a grade, foco devolvido à linha) e reescrever `Inbox`. A chamada de aceitar continua `POST /inbox/{id}/accept` seguida, se pedido, de `POST /rules`.
- [ ] **Step 4: Rodar a suíte inteira.** Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): Caixa as a grouped list with a decision sheet`.

---

### Task 5: e2e, capturas e fechamento

**Files:**
- Modify: `web/e2e/app.spec.ts`

- [ ] **Step 1: Ajustar os e2e:** o fluxo "settings and recurring" chega às recorrências por Lançamentos (a seção "Repetem todo mês"), pausa com o interruptor `Pausar Netflix` e confere "pausada"; o fluxo do OFX decide as linhas da Caixa pela folha (tocar na linha, escolher a categoria na grade, "Aceitar"; e "Ignorar").
- [ ] **Step 2: Novos e2e:** abrir `/recorrentes` leva a Lançamentos; editar o valor de uma recorrência pela folha e ver o novo valor na seção; e, no desktop a 900 px, Lançamentos e Caixa não têm rolagem lateral (mesma checagem do PR 2, com dados longos).
- [ ] **Step 3: Rodar** `npm run build && npm run e2e`. Esperado: todos passam, nos dois projetos.
- [ ] **Step 4: Capturas** de Lançamentos (com a seção de recorrentes) e da Caixa, celular e desktop, claro e escuro, com um spec temporário que não é commitado; corrigir o que destoar do desenho aprovado.
- [ ] **Step 5: Abrir o PR** descrevendo o que mudou e que o merge dispara o deploy (a branch `master` exige `--admin`).
- [ ] **Step 6: Commit** `test(e2e): recurrences inside Lançamentos and the Caixa sheet`.
