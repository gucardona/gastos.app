# Reestruturação da interface, PR 5: Divisão, login e acabamento

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Terminar a reestruturação: a Divisão e o login na linguagem nova, o fim do CSS e das classes do sistema antigo, e uma regra automática que impede o texto de voltar a ficar centralizado por tela.

**Architecture:** A Divisão vira uma frase grande ("Bia deve R$ 120,00 a você") com a ação "Registrar acerto", o saldo de cada pessoa em lista agrupada e o histórico em lista agrupada; criar e editar um acerto acontece numa folha (`SettlementSheet`), no mesmo padrão das folhas dos PRs 2 a 4. O login troca os campos soltos por linhas de campo no estilo das folhas. A passada final apaga o que ficou sem uso (`.item`, `.line`, `.inline-form`, `.text`, `.field`, `.chip` e a variável `--display`) e um teste lê o CSS e falha se aparecer um `text-align: center` fora da lista de exceções da spec.

**Tech Stack:** React 19, React Router 8, Vitest + Testing Library, Playwright (desktop e Pixel 5), CSS puro com os tokens do PR 1.

**Spec:** [2026-10-11-ui-redesign-design.md](../specs/2026-10-11-ui-redesign-design.md), seções Divisão, Login e cadastro, Alinhamento e Entrega (PR 5). Base: PRs 1 a 4 (o PR 4 está aberto: esta branch parte dele).

## Global Constraints

- Código, comentários e testes em inglês; texto da interface em pt-BR.
- Mesmo sistema dos PRs anteriores: blocos agrupados de raio 22, controles de raio 12, sem sombras, texto à esquerda e valores à direita, alvos de toque de pelo menos 44 px, foco visível (inclusive nos campos que escondem o contorno do navegador), contraste AA nos dois temas, só tokens de `tokens.css`.
- **Alinhamento (regra do sistema):** centralizado só em estados vazios (`.empty`), na pré-visualização de ícone (`.preview`), no valor da folha de lançamento (`.amount`), nos blocos de categoria (`.cat-tile`), no cabeçalho de escolha (`.picker-head`) e no emoji (`.emoji-grid`). Qualquer outro `text-align: center` é erro.
- Confirmação de exclusão: o botão de confirmar é outro elemento (`key` diferente), para um clique duplo não confirmar.
- Divisão: só existe para carteira que divide gastos (como hoje); o texto de quem deve a quem usa "Você" para o próprio usuário e o primeiro nome dos outros.
- Leitor: vê tudo e não registra, edita nem exclui nada.
- A API, os dados e as regras não mudam. Todo comportamento coberto hoje por `Division.test.tsx` e `Login.test.tsx` continua coberto.

## Review Focus

- Acerto sugerido com 3 ou mais pessoas: cada transferência sugerida aparece com seu próprio botão e nome certo; "Vocês estão quites" quando não há sugestão. Task 1.
- Acerto manual: valor inválido ou zero e pagador igual a recebedor são recusados antes de enviar; editar mantém a data e a observação; excluir só depois de confirmar, e um clique duplo não confirma. Task 1.
- Leitor e carteira sem divisão: nenhuma ação de escrita; o dono vê "Ligar a divisão" levando a `/ajustes/carteira`. Task 1.
- Login: senha errada, cadastro fechado, código de convite, senhas diferentes e erros de campo continuam com a mesma mensagem; Enter envia; os atributos `autocomplete` e `type` dos campos permanecem (gerenciadores de senha). Task 2.
- Ajustes em tela larga sem seção escolhida mostra o Perfil no painel direito (não um painel vazio), e há um só `h1` visível por tela. Task 3.
- Nenhum `text-align: center` fora da lista da regra de alinhamento, e nenhuma classe antiga resta no CSS. Task 3.
- Sem rolagem lateral entre 900 e 1100 px na Divisão, com nomes longos. Task 4.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade |
|---|---|
| `web/src/pages/Division.tsx` (reescreve) | Frase de quem deve a quem, saldos, histórico |
| `web/src/components/SettlementSheet.tsx` (novo) | Folha de criar, editar e excluir um acerto |
| `web/src/pages/Login.tsx` (modifica) | Linhas de campo no estilo novo |
| `web/src/pages/Settings.tsx` (modifica) | Perfil como painel padrão em tela larga; um só `h1` visível |
| `web/src/styles.css` (modifica) | Apaga o CSS do sistema antigo e a variável `--display` |
| `web/src/styles/tokens.css` (modifica) | Remove `--display` |
| `web/src/styles/alignment.test.ts` (novo) | Guarda da regra de alinhamento e das classes antigas |

---

### Task 1: Divisão

**Files:**
- Create: `web/src/components/SettlementSheet.tsx`, `web/src/components/SettlementSheet.test.tsx`
- Modify: `web/src/pages/Division.tsx`, `web/src/pages/Division.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: `Sheet`, `ListGroup`, `ListRow`, `PickerView`, `OptionList`, `useAction`.
- Produces: `function SettlementSheet(props: { settlement: Settlement | "new" | null; onClose: () => void }): JSX.Element`. Linhas **Quem pagou** e **Quem recebeu** (cada uma abre `OptionList` com os membros), campos `Valor do acerto`, `Data do acerto` (data nativa) e `Observação do acerto`, botão Salvar (ou Registrar, ao criar) e, ao editar, "Excluir acerto" com confirmação. Mensagens: "Informe um valor maior que zero." e "Escolha pessoas diferentes para quem paga e quem recebe."

A tela: título "Divisão"; para cada transferência sugerida, uma frase grande (`<p class="big">`: "Bia deve R$ 120,00 a você") e o botão "Registrar acerto" (só para quem edita); sem sugestão, "Vocês estão quites."; "Registrar outro acerto" abre a folha; o saldo de cada pessoa em `ListGroup` (nome, "tem a receber", "deve" ou "está quite", valor); o histórico em `ListGroup` (quem pagou a quem, data e observação, valor), tocar numa linha abre a folha de edição.

- [ ] **Step 1: Testes** portando os 11 comportamentos de `Division.test.tsx` (explica e oferece ligar a divisão quando desligada, sem o botão para quem não administra; saldo de cada pessoa e o que falta acertar; "quites" sem sugestão; registra o acerto sugerido com a data de hoje; registra outro acerto à mão validando valor e pessoas; histórico com data, valor e observação; edita um acerto do histórico por PUT; exclui só depois de confirmar; leitor só olha; erro de carga com nova tentativa) pelos componentes novos, e `SettlementSheet.test.tsx`: criar envia POST com `fromUserId`, `toUserId`, `amountCents`, `date`, `note` aparada; editar vem preenchido e envia PUT preservando data e observação; valor zero ou texto e pagador igual a recebedor são recusados sem chamar a API; excluir exige confirmação e um clique duplo não confirma; erro da API aparece sem fechar a folha. Acrescentar: três pessoas com duas transferências sugeridas mostram dois botões "Registrar acerto" e cada um envia a sua.
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar** a folha e a tela (a lógica de `useAction`, de `api` e das mensagens permanece; sai o formulário inline).
- [ ] **Step 4: Rodar a suíte inteira.** Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): Divisão with a sentence, grouped lists and a settlement sheet`.

---

### Task 2: Login e cadastro

**Files:**
- Modify: `web/src/pages/Login.tsx`, `web/src/pages/Login.test.tsx`, `web/src/styles.css`

**Interfaces:**
- Consumes: nenhuma novidade; os campos mantêm `id`, `type`, `autoComplete`, os rótulos e as mensagens de hoje.

- [ ] **Step 1: Testes** em `Login.test.tsx`: manter todos os comportamentos (entra e vai à Visão, senha errada, cadastro aberto sem código, com código de convite, código recusado, cadastro fechado, sem oferecer cadastro até saber o modo, ainda oferece se o modo não carrega, cria a conta e mostra erros de campo, recusa senhas diferentes, não pede repetir a senha ao entrar) e acrescentar: os campos têm `autocomplete` `email`, `current-password` ao entrar e `new-password` (duas vezes) ao cadastrar, e `name` no nome; o campo de e-mail é `type="email"` e os de senha `type="password"`; Enter no último campo envia o formulário; o botão fica desativado durante o envio.
- [ ] **Step 2: Rodar e ver falhar** o que for novo.
- [ ] **Step 3: Implementar** as linhas de campo (rótulo à esquerda, valor à direita, no estilo `.field-row` das folhas, com o foco desenhado na linha), título "gastos", um botão principal de largura total e o alternador de modo como texto; erros de campo sob a linha.
- [ ] **Step 4: Rodar a suíte inteira.** Esperado: verde.
- [ ] **Step 5: Commit** `feat(web): sign-in and sign-up with field rows`.

---

### Task 3: Acabamento

**Files:**
- Create: `web/src/styles/alignment.test.ts`
- Modify: `web/src/styles.css`, `web/src/styles/tokens.css`, `web/src/styles/tokens.test.ts`, `web/src/pages/Settings.tsx`, `web/src/pages/SettingsPage.test.tsx`, e as páginas que ainda usam `var(--display)` ou estilos inline de título (`Overview.tsx`, `Transactions.tsx`, `Inbox.tsx`)

- [ ] **Step 1: Testes.** `alignment.test.ts` lê `styles.css` e falha se houver `text-align: center` em qualquer regra cujo seletor não esteja na lista da Global Constraints; falha também se o CSS ainda definir `.item`, `.item-actions`, `.inline-form`, `.inline`, `.line`, `.settings`, `.text`, `.field`, `.chip`, `.seg` ou `--display`. `tokens.test.ts`: o contrato de tokens não exige mais `--display`. `SettingsPage.test.tsx`: em `/ajustes` (sem seção) o painel de detalhe padrão mostra o Perfil (nome e e-mail) e fica escondido no celular por CSS; com uma seção aberta há um só `h1` na página (o título do índice passa a `h2`).
- [ ] **Step 2: Rodar e ver falhar.**
- [ ] **Step 3: Implementar:** painel padrão de Perfil em tela larga; título do índice como `h2` quando há seção; trocar os títulos inline restantes pela classe `.page-title`; apagar as regras e a variável `--display` (ajustar quem a usa para `--font`); conferir com `grep` em `src/` que nenhuma classe apagada é usada.
- [ ] **Step 4: Rodar a suíte inteira, `tsc`, `lint` e `build`.** Esperado: verde.
- [ ] **Step 5: Commit** `chore(web): drop the old system's CSS and guard the alignment rule`.

---

### Task 4: e2e, capturas e fechamento

**Files:**
- Modify: `web/e2e/app.spec.ts`

- [ ] **Step 1: Ajustar os e2e** da Divisão e do cadastro aos controles novos (folha de acerto, linhas de campo).
- [ ] **Step 2: Novos e2e:** registrar outro acerto à mão pela folha e vê-lo no histórico; na carteira com divisão, a 900 px e com nomes longos, a Divisão não rola para o lado; o login por teclado (Tab até o botão, Enter envia).
- [ ] **Step 3: Rodar** `npm run build && npm run e2e`. Esperado: todos passam, nos dois projetos.
- [ ] **Step 4: Capturas** da Divisão e do login (celular e desktop, claro e escuro) com um spec temporário não commitado; corrigir o que destoar.
- [ ] **Step 5: Abrir o PR** (base: a branch do PR 4 enquanto o #30 não for mergeado), avisando que o merge dispara o deploy e exige `--admin`.
- [ ] **Step 6: Commit** `test(e2e): Divisão and sign-in in the new system`.
