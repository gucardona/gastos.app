# Reestruturação da interface, PR 4: Ajustes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refazer Ajustes como duas listas curtas em que cada linha abre uma tela só dela, com seletores visuais de ícone e de cor, e folhas no lugar dos formulários soltos.

**Architecture:** `/ajustes` vira um índice (listas agrupadas) e `/ajustes/:section` a tela de cada assunto. No celular o índice e a tela se alternam, com um link "‹ Ajustes" para voltar; a partir de 900 px aparecem lado a lado (lista à esquerda, conteúdo à direita). Cada seção troca o formulário inline por linhas de lista e uma folha (`Sheet`) de criação e edição, reusando `Segmented`, `ListGroup`, `Switch` e os pickers do PR 2. Dois componentes novos entram: `EmojiPicker` (grade com abas por assunto) e `ColorSwatches` (fileira de amostras). Recorrentes passa a ter também uma tela em Ajustes, reusando `RecurringSection`.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop e Pixel 5), CSS puro com os tokens do PR 1.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), seção Ajustes (Pessoas, Categorias, Contas bancárias, Aparência), Sistema de design e Entrega (PR 4). Base: PRs 1 a 3 no master.

## Global Constraints

- Código, comentários e testes em inglês; texto da interface em pt-BR.
- Mesmo sistema dos PRs 2 e 3: blocos agrupados de raio 22, controles de raio 12, sem sombras, texto à esquerda e valores à direita, alvos de toque de pelo menos 44 px, foco visível, contraste AA nos dois temas, e só tokens de `tokens.css`. As amostras de cor das categorias são dado do usuário (uma paleta fixa de 8 cores definida no componente), não tokens de interface.
- Índice de Ajustes, nessa ordem. Grupo sem título: **Perfil**. Grupo "Casa": **Carteira**, **Pessoas**, **Categorias**, **Formas de pagamento**, **Orçamentos**. Grupo "Dados": **Contas bancárias**, **Recorrentes**, **Aparência**. Por último, **Sair** (texto na cor de alerta, sem confirmação, funciona mesmo offline como hoje).
- Rotas: `/ajustes` e `/ajustes/:section` com `section` em `perfil`, `carteira`, `pessoas`, `categorias`, `pagamentos`, `orcamentos`, `bancos`, `recorrentes`, `aparencia`. O endereço antigo `/ajustes?s=<id>` redireciona (`conta` para `perfil`, `membros` para `pessoas`; os demais ids são os mesmos). Seção desconhecida volta para `/ajustes`.
- Tipo de gasto com os rótulos da spec: Essencial | Supérfluo | Investimento (valores da API: `essential`, `nonessential`, `investment`).
- Leitor e quem não administra: tudo visível e nada editável, como hoje (cada controle de escrita some ou fica desativado conforme o papel).
- Exclusões pedem confirmação no mesmo lugar; excluir uma categoria com lançamentos arquiva e explica ("Arquivada, porque já tem lançamentos.").
- A API, os dados e as regras não mudam. Todo comportamento hoje coberto por `Settings.test.tsx`, `SettingsPage.test.tsx` e `BankSection.test.tsx` continua coberto (mesmos comportamentos, seletores atualizados).

## Review Focus

- Quem tinha `/ajustes?s=bancos` salvo (e o link da Caixa vazia) cai na tela de contas bancárias; id desconhecido não deixa tela em branco. Task 1.
- Categoria editada cujo ícone não está na grade (digitado livre na versão antiga): o ícone aparece selecionado e não é perdido ao salvar. Task 2 e 3.
- Categoria ou forma de pagamento arquivada: aparece sob "Arquivadas", restaura, e a exclusão de uma em uso vira arquivamento com a mensagem. Task 3.
- Quem não é administrador: não convida, não remove, mas pode sair da carteira; leitor não edita nada em nenhuma seção. Task 4.
- Orçamento: valor inválido ou zero é recusado antes de enviar; remover limpa o campo. Task 4.
- Carteira: criar uma nova passa a usá-la; excluir só depois de confirmar, com o aviso quando é a única. Task 4.
- Contas bancárias: importar OFX e CSV (mapeamento de colunas só no CSV), conta desativada não importa, regras listadas e removíveis. Task 5.
- Sem rolagem lateral entre 900 e 1100 px em nenhuma seção, com nomes longos (categoria, conta, e-mail). Task 6.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `web/src/pages/Settings.tsx` (reescreve) | Índice, layout mestre e detalhe, redirecionamento do endereço antigo, Perfil, Aparência e Sair |
| `web/src/pages/settings/` (modifica) | Uma tela por assunto, cada uma com sua folha: `WalletSection`, `MembersSection`, `CategoriesSection`, `PaymentsSection`, `BudgetsSection`, `BankSection` |
| `web/src/components/EmojiPicker.tsx` (novo) | Grade de emojis com abas por assunto |
| `web/src/components/ColorSwatches.tsx` (novo) | Fileira de amostras de cor |
| `web/src/pages/Inbox.tsx` (modifica) | O link da Caixa vazia aponta para `/ajustes/bancos` |
| `web/src/styles.css` (modifica) | Estilos novos; remove `.item`, `.item-actions`, `.inline-form`, `.settings`, `.line`, `.seg`, `.chip` e `.filters` quando `grep` mostrar que nada mais os usa |

---

### Task 1: Índice, rotas e telas simples

**Files:**
- Modify: `web/src/pages/Settings.tsx`, `web/src/pages/SettingsPage.test.tsx`, `web/src/App.tsx`, `web/src/pages/Inbox.tsx`, `web/src/pages/Inbox.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `ListGroup`, `ListRow`, `Segmented`, `RecurringSection`, `PageBar`.
- Produces: `function Settings(): JSX.Element` (layout), `export const SETTINGS_SECTIONS` com `{ id, label, group }` para as nove seções; as telas de cada seção continuam sendo componentes `*Section` sem props, renderizados pelo layout conforme `useParams().section`.

- [ ] **Step 1: Testes** em `SettingsPage.test.tsx`: `/ajustes` mostra as três listas na ordem da Global Constraints, com o nome e o e-mail no Perfil como valor, e "Sair" por último; cada linha leva a `/ajustes/<id>` (`href`); abrir `/ajustes/aparencia` mostra o `Segmented` "Tema" (Automático, Claro, Escuro) que aplica e lembra a escolha (mesmos comportamentos de hoje); `/ajustes/perfil` mostra nome, e-mail e o papel na carteira; `/ajustes/recorrentes` mostra a seção "Repetem todo mês"; `/ajustes?s=bancos` redireciona para `/ajustes/bancos`, `?s=conta` para `/ajustes/perfil`, `?s=membros` para `/ajustes/pessoas`; `/ajustes/inexistente` redireciona para `/ajustes`; em qualquer seção há o link "‹ Ajustes" para voltar; Sair encerra a sessão e leva ao login, também quando a chamada falha. Em `Inbox.test.tsx`, o link da caixa vazia tem `href="/ajustes/bancos"`.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** o layout: o índice e o detalhe lado a lado a partir de 900 px (a linha ativa destacada), alternados no celular; Perfil, Aparência (com `Segmented`) e Sair no próprio `Settings.tsx`; as seções ainda não migradas (`carteira`, `pessoas`, `categorias`, `pagamentos`, `orcamentos`, `bancos`) continuam com o conteúdo atual dentro do novo layout. Atualizar o link da Caixa e a rota (`<Route path="ajustes/:section?" ...>`).
- [ ] **Step 4: Rodar a suíte inteira** (`vitest`, `tsc`, `lint`). Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): Ajustes as an index with a screen per subject`.

---

### Task 2: Seletores de ícone e de cor

**Files:**
- Create: `web/src/components/EmojiPicker.tsx`, `web/src/components/EmojiPicker.test.tsx`, `web/src/components/ColorSwatches.tsx`, `web/src/components/ColorSwatches.test.tsx`
- Modify: `web/src/styles.css`

**Interfaces:**
- Produces:
  - `function EmojiPicker(props: { value: string; onPick: (emoji: string) => void }): JSX.Element`: abas Casa, Comida, Transporte, Lazer, Saúde, Dinheiro e Outros (botões com `aria-pressed`), e uma grade de 6 colunas de botões cujo nome acessível é o próprio emoji. Mostra a aba que contém `value`; se `value` não está em nenhuma aba, aparece como primeira opção da grade, marcada.
  - `function ColorSwatches(props: { value: string; onPick: (hex: string) => void }): JSX.Element`: 8 amostras (`#0E6577`, `#3B6FD8`, `#7A5AD6`, `#D4568B`, `#D9822B`, `#2F9D68`, `#8B949A`, `#C2412D`) como botões `aria-pressed` com nome `Cor <hex>`; se `value` não é uma delas (cor antiga), entra como nona amostra marcada.

- [ ] **Step 1: Testes:** `EmojiPicker` abre na aba do `value` (ex.: 🛒 abre em Comida? definir o mapeamento no teste com os emojis reais das abas), troca de aba mostra os emojis dela, `onPick` recebe o emoji tocado, só o `value` fica `aria-pressed`, e um emoji fora das abas aparece marcado como primeira opção; cada aba tem pelo menos 12 emojis e nenhum emoji se repete entre abas (guarda de dados). `ColorSwatches`: 8 amostras, só a de `value` marcada, `onPick` recebe o hex, e uma cor fora da paleta vira a nona amostra marcada.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** os dois componentes e o CSS (amostra redonda de 32 px com área de toque de 44 px, anel `--accent` na marcada; abas em rolagem horizontal discreta; grade de 6 colunas com botões de 44 px).
- [ ] **Step 4: Rodar e ver passar.**
- [ ] **Step 5: Commit** `feat(web): emoji picker and color swatches`.

---

### Task 3: Categorias e Formas de pagamento

**Files:**
- Create: `web/src/components/CategorySheet.tsx`, `web/src/components/CategorySheet.test.tsx`, `web/src/components/PaymentSheet.tsx`, `web/src/components/PaymentSheet.test.tsx`
- Modify: `web/src/pages/settings/CategoriesSection.tsx`, `web/src/pages/settings/PaymentsSection.tsx`, `web/src/pages/settings/Settings.test.tsx`

**Interfaces:**
- Consumes: `Sheet`, `Segmented`, `ListGroup`, `ListRow`, `EmojiPicker`, `ColorSwatches`, `useAction`.
- Produces:
  - `function CategorySheet(props: { category: Category | "new" | null; kind: Kind; onClose: () => void }): JSX.Element`: fechada com `null`; `"new"` cria na categoria de tipo `kind`; um `Category` edita. Conteúdo: pré-visualização (ícone e nome grandes), campo `Nome`, `EmojiPicker`, `ColorSwatches`, e para gasto o `Segmented` "Tipo de gasto" (Essencial, Supérfluo, Investimento); botões Salvar (ou Criar categoria), e na edição Arquivar ou Restaurar e "Excluir categoria" com confirmação.
  - `function PaymentSheet(props: { method: PaymentMethod | "new" | null; onClose: () => void }): JSX.Element`: mesmo padrão, com `Nome` e `EmojiPicker`.

A tela de Categorias: `Segmented` "Tipo de categoria" (Gastos | Entradas), um `ListGroup` com uma linha por categoria ativa (ícone, nome, tipo de gasto como valor, amostra de cor), botão "Nova categoria" e a seção recolhível "Arquivadas (N)". Tocar numa linha abre `CategorySheet`; leitor vê as linhas sem abrir nada.

- [ ] **Step 1: Testes** portando os comportamentos de `Settings.test.tsx` (categorias: cria gasto com tipo de gasto, entrada sem tipo de gasto, mostra o erro da API como nome duplicado, explica quando a exclusão virou arquivamento, restaura arquivada; formas de pagamento: cria uma) e acrescentando: editar envia PUT com o corpo completo (`kind`, `name`, `icon`, `color`, `essentiality`, `sortOrder`, `archived`); um ícone fora da grade continua selecionado e é enviado ao salvar; nome vazio é recusado pela API e o erro aparece sem fechar a folha; leitor não abre a folha; a lista mostra a cor da categoria.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** as duas folhas e reescrever as duas telas (a lógica de `body()` e de exclusão que arquiva permanece).
- [ ] **Step 4: Rodar a suíte inteira.** Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): categories and payment methods with sheets and visual pickers`.

---

### Task 4: Pessoas, Carteira e Orçamentos

**Files:**
- Create: `web/src/components/MemberSheet.tsx`, `web/src/components/MemberSheet.test.tsx`, `web/src/components/BudgetSheet.tsx`, `web/src/components/BudgetSheet.test.tsx`
- Modify: `web/src/pages/settings/MembersSection.tsx`, `web/src/pages/settings/WalletSection.tsx`, `web/src/pages/settings/BudgetsSection.tsx`, `web/src/pages/settings/Settings.test.tsx`

**Interfaces:**
- Produces:
  - `function MemberSheet(props: { member: Member | null; onClose: () => void }): JSX.Element`: para o administrador, `Segmented` "Permissão" (Pode editar | Só leitura) de outro membro, `Switch` "Dividir gastos com <nome> por padrão" quando a carteira divide, e remover; para qualquer membro não administrador, "Sair da carteira" (só na própria linha). Confirmação nas duas saídas.
  - `function BudgetSheet(props: { category: Category | null; current?: number; onClose: () => void }): JSX.Element`: campo `Limite mensal de <categoria>`, Definir e, se há limite, Remover.

Telas: **Pessoas** (membros em uma lista; tocar abre `MemberSheet`; para o administrador uma linha "Convidar alguém" abre uma folha com `E-mail do convidado`, `Segmented` "Permissão do convidado" e o botão Convidar); **Carteira** (linha `Nome da carteira` editável só pelo administrador com "Salvar nome", `Switch` "Dividir gastos entre os membros", linha "Nova carteira" que abre uma folha com nome e o interruptor, e "Excluir esta carteira" em cor de alerta com confirmação e o aviso de única carteira); **Orçamentos** (uma linha por categoria de gasto ativa, o limite como valor; tocar abre `BudgetSheet`).

- [ ] **Step 1: Testes** portando os comportamentos de `Settings.test.tsx` (membros: convida por e-mail com a permissão escolhida, mostra o erro quando o e-mail não tem conta, o administrador muda a permissão de outro mas não a própria, quem não administra não convida nem remove mas pode sair; orçamentos: mostra o limite e salva em centavos, recusa valor inválido sem chamar a API, remove o limite, lista só categorias de gasto ativas; carteira: liga a divisão, cria nova e passa a usá-la, exclui só depois de confirmar, quem não administra não edita) pelas folhas novas, mais: `Segmented` de permissão envia `PATCH` com `{ role }`; o interruptor de divisão padrão envia `{ splitsByDefault }`; confirmar a saída da carteira envia `DELETE` do próprio membro; zero e texto no limite mostram "Informe um valor maior que zero.".
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** as duas folhas e reescrever as três telas.
- [ ] **Step 4: Rodar a suíte inteira.** Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): people, wallet and budgets with sheets`.

---

### Task 5: Contas bancárias

**Files:**
- Create: `web/src/components/BankAccountSheet.tsx`, `web/src/components/BankAccountSheet.test.tsx`
- Modify: `web/src/pages/settings/BankSection.tsx`, `web/src/pages/settings/BankSection.test.tsx`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `Switch`, `OptionList`, `PickerView`, `Segmented`, `useAction`, `api.upload`.
- Produces: `function BankAccountSheet(props: { account: BankAccount | "new" | null; onClose: () => void }): JSX.Element`: nova (só o nome) ou existente (forma de pagamento dos lançamentos como linha que abre `OptionList`, `Switch` "Conta ativa", **Importar extrato** com o formato num `Segmented` (OFX | CSV), o arquivo e, só no CSV, as colunas de data, descrição e valor, o cabeçalho e "valores positivos são compras", e "Excluir conta" com confirmação).

A tela: lista de contas (nome, "N para revisar", "em dia" ou "desativada"), "Nova conta bancária", e abaixo "Regras de categoria" (cada regra com o texto e a categoria; remover com um toque). Importar mostra "N linhas novas, M já importadas antes." como hoje.

- [ ] **Step 1: Testes** portando os 8 de `BankSection.test.tsx` (lista com o que espera revisão; cria conta; envia OFX e relata o que foi novo; pede as colunas do CSV e as envia; não importa sem arquivo; mostra por que o arquivo foi recusado; troca a forma de pagamento mantendo os outros campos; exclui uma regra) pela folha, e acrescentando: conta desativada não oferece importar; o formato CSV mostra os campos de coluna e o OFX não; remover a conta só depois de confirmar.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** a folha e a tela (a lógica do envio, `FormData` e as mensagens permanecem).
- [ ] **Step 4: Rodar a suíte inteira.** Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): bank accounts with a sheet and the import form inside it`.

---

### Task 6: e2e, limpeza e fechamento

**Files:**
- Modify: `web/e2e/app.spec.ts`, `web/src/styles.css`

- [ ] **Step 1: Ajustar os e2e** que usam Ajustes: o fluxo "settings and recurring" (nova categoria com nome, ícone da grade, cor e tipo de gasto; definir orçamento; ver o estouro na Visão), o de OFX (criar a conta e importar pela folha), o de sair (linha "Sair") e o de navegação (um só "Ajustes").
- [ ] **Step 2: Novos e2e:** abrir `/ajustes?s=bancos` leva a `/ajustes/bancos`; convidar uma pessoa pela folha mostra o erro quando o e-mail não existe; e, a 900 px com nomes longos (categoria, conta bancária), nenhuma seção rola para o lado (mesmo padrão dos PRs 2 e 3).
- [ ] **Step 3: Limpeza de CSS:** apagar as classes que `grep` mostrar sem uso (`.item`, `.item-actions`, `.inline-form`, `.settings`, `.line`, `.seg`, `.chip`, `.filters`, `.swatch` e afins), conferindo cada uma em `src/` antes.
- [ ] **Step 4: Rodar** `npm run build && npm run e2e`. Esperado: todos passam, nos dois projetos.
- [ ] **Step 5: Capturas** do índice e de três seções (Categorias com a folha, Pessoas, Contas bancárias), celular e desktop, claro e escuro, com um spec temporário que não é commitado; corrigir o que destoar.
- [ ] **Step 6: Abrir o PR**, avisando que o merge dispara o deploy e hoje exige `--admin`.
- [ ] **Step 7: Commit** `test(e2e): the new Ajustes`.
