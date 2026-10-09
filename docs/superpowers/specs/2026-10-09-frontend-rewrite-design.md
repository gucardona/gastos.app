# Frontend — reescrita em React

**Data:** 2026-10-09
**Status:** Draft — a direção visual precisa de aprovação por mockup antes da implementação
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

## Objetivo

Substituir o `src/web/index.html` (165 KB, Alpine + Chart.js + flatpickr via CDN) por um app React tipado, minimalista e robusto, consumindo a API v2 da [spec de backend](2026-10-09-backend-foundation-design.md).

Robusto aqui quer dizer:
- tipos de ponta a ponta;
- estados de loading, erro e vazio em toda tela;
- sem perder dados de formulário;
- funciona bem no celular.

Minimalista quer dizer:
- menos telas e menos cliques para o caso comum, que é lançar um gasto;
- hierarquia tipográfica no lugar de cards coloridos;
- uma cor de destaque só.

## Stack

| Camada | Escolha | Motivo |
|---|---|---|
| Build | Vite + TypeScript (strict) | rápido, padrão |
| UI | React 19 | |
| Estilo | Tailwind CSS v4 + componentes no estilo shadcn/ui (Radix por baixo) | acessibilidade pronta (dialog, popover, select), código dos componentes fica no repo |
| Rotas | TanStack Router | search params tipados: `?m=2026-10&cat=3` vira estado validado na URL |
| Dados | TanStack Query | cache, invalidação por SSE, updates otimistas |
| Formulários | react-hook-form + zod | validação igual ao backend, erros de campo vindos da API mapeados |
| Gráficos | Recharts | poucos gráficos, todos simples |
| Datas | date-fns (locale `pt-BR`) | |
| Ícones | lucide-react | as categorias continuam aceitando emoji como ícone |
| Testes | Vitest + Testing Library; Playwright para smoke e2e | |

Sem gerenciador de estado global: o estado do servidor fica no TanStack Query, o da URL no router e o resto em `useState`.

## Estrutura

```
web/
  index.html
  embed.go                 # package web; //go:embed all:dist
  src/
    main.tsx, router.tsx
    api/                   # client fetch (credentials:'include'), tipos, hooks de query por recurso
    lib/money.ts           # centavos ↔ string pt-BR, parse de input "12,90"
    lib/dates.ts
    components/ui/         # primitives (button, input, dialog, sheet, select, combobox, toast...)
    components/            # MoneyInput, CategoryPicker, MonthSwitcher, TransactionRow, EmptyState...
    features/
      auth/  overview/  transactions/  recurring/  budgets/  split/  settings/  inbox/ (fase 4)
    styles.css             # tokens (CSS vars) + tailwind
```

## Arquitetura de informação

Antes eram 7 abas: dashboard, expenses, income, payments, goals, charts e settings. Agora são **5 destinos**:

| Rota | Conteúdo | Substitui |
|---|---|---|
| `/` **Visão geral** | resumo do mês: saldo, entradas, saídas, média diária, comparativo com o mês anterior; gastos por categoria com barra de orçamento; últimos lançamentos; "acertos" se a divisão estiver ativa | dashboard + charts + goals (visualização) |
| `/lancamentos` | lista única de gastos e entradas do mês, agrupada por dia; filtros (tipo, categoria, forma de pagamento, busca); totais no topo | expenses + income |
| `/recorrentes` | modelos recorrentes (gasto, entrada, acerto) com toggle on/off e próxima data | os três blocos "recorrentes" espalhados |
| `/divisao` | saldos entre membros, acertos sugeridos (com botão "registrar acerto"), histórico de acertos; só aparece com `splittingEnabled` | payments |
| `/ajustes` | carteira (nome, divisão), membros, categorias, formas de pagamento, orçamentos, conta (logout), tema | settings + goals (edição) |

Na fase 4 entra a **`/caixa`** (caixa de entrada de transações importadas), com um badge de pendências.

O mês selecionado fica na URL (`?m=2026-10`) e é compartilhado entre Visão geral e Lançamentos. O seletor de carteira fica no topo do layout; a carteira ativa fica em `localStorage`, com fallback para a primeira carteira de `/api/me`.

## Lançamento rápido

É o fluxo mais importante do app.

- O botão "+" (FAB no mobile, botão no header no desktop) e o atalho `N` abrem um **sheet** (de baixo no mobile, lateral no desktop).
- A ordem dos campos é pensada para o polegar:
  - toggle **Gasto / Entrada**;
  - **valor** com teclado numérico (`inputmode="decimal"`), já focado;
  - **descrição**;
  - **categoria** em grade de chips com as mais usadas primeiro;
  - forma de pagamento;
  - data (padrão hoje, com atalhos "ontem" e "hoje");
  - "quem pagou" e split, só se a divisão estiver ativa, recolhidos em "Dividir: igual entre 2 ▾".
- O toggle "Repetir todo mês" transforma o lançamento em recorrência sem trocar de tela.
- `Enter` salva; "Salvar e lançar outro" mantém o sheet aberto com tipo, categoria e data preservados.
- O update é otimista: o item aparece na lista na hora e reverte com um toast de erro se a API falhar.
- O rascunho fica em `sessionStorage` enquanto o sheet está aberto. Fechar sem querer não perde o que foi digitado.

Editar usa o mesmo sheet. Excluir tem um "Desfazer" no toast (5 segundos) antes de chamar a API.

## Tempo real

`EventSource('/api/wallets/{id}/events')` com o cookie, sem token. A cada `changed`, chama `queryClient.invalidateQueries({ queryKey: ['wallet', id] })`. O hook reconecta sozinho e fecha ao trocar de carteira.

## Direção visual (proposta, a aprovar)

- **Tom:** calmo e editorial, como um extrato bem diagramado, não um painel de banco.
- **Base neutra:** off-white no claro (`#FAFAF9`), grafite no escuro (`#0C0C0D`).
- **Uma cor de destaque:** verde-petróleo, herdando o `#0f6579` atual, mais saturado no escuro.
- **Semântica:** saída em neutro e entrada em verde. Vermelho só para estouro de orçamento e erros, para não pintar a tela de vermelho num mês normal.
- **Tipografia:** Inter (UI) com `font-variant-numeric: tabular-nums` em todo valor. Valores grandes do resumo em peso 600, tamanho 32–40px. Sem fonte decorativa.
- **Densidade:** listas com linhas de 52–56px:
  - ícone da categoria;
  - descrição e subtítulo (categoria · forma de pagamento);
  - valor alinhado à direita.
  Sem cards dentro de cards.
- **Gráficos:**
  - barras horizontais de categoria × orçamento, que substituem a pizza;
  - uma linha de gasto acumulado no mês contra o mês anterior.
  Só esses dois.
- **Layout:**
  - mobile-first;
  - bottom nav com 4 itens + FAB (Ajustes fica no menu do avatar);
  - no desktop, sidebar estreita e conteúdo com largura máxima de 960px.
- **Tema:** claro, escuro ou sistema, em CSS vars com `data-theme`.
- **Movimento:** transições de 150ms em sheet, dialog e toast; respeita `prefers-reduced-motion`.

**Antes de implementar:** produzir um mockup estático (HTML) de Visão geral, Lançamentos e do sheet de lançamento rápido, em claro e escuro e em 375px e 1280px, para aprovação.

## Contrato com a API

- `api/client.ts`:
  - `fetch` com `credentials: 'include'`;
  - um `401` em qualquer chamada leva para `/login?next=...`;
  - erros viram `ApiError {code, message, fields}`.
- Os tipos ficam em `api/types.ts`, escritos à mão espelhando a spec do backend. Se o volume crescer, migrar para OpenAPI gerado.
- Dinheiro é sempre `number` em centavos. Só o `MoneyInput` e o `formatMoney` lidam com reais.

## Acessibilidade e qualidade

- Navegação completa por teclado. Foco visível. Contraste AA nos dois temas.
- Todo dialog e sheet com focus trap (Radix).
- `EmptyState` em toda lista: primeira carteira, mês sem lançamentos, sem recorrências.
- Error boundary por rota, com "tentar novamente".
- PWA mínimo: manifest e ícones, para "adicionar à tela inicial" no celular, reaproveitando os ícones atuais. Sem service worker offline por ora.

## Testes

- **Vitest:**
  - `lib/money` (parse e format, incluindo "1.234,56", "12,9" e "0,05");
  - mapeamento de erros de campo;
  - lógica de split no formulário.
- **Testing Library:** sheet de lançamento rápido (validação, salvar e lançar outro, rascunho).
- **Playwright (smoke contra o binário real com DB temporário):**
  - registrar;
  - lançar gasto e entrada;
  - ver o resumo;
  - criar recorrência;
  - convidar um membro e dividir um gasto.

## Critérios de aceite

- Paridade funcional com o `index.html` atual. Checklist derivado das features:
  - auth;
  - carteiras e membros com papéis;
  - categorias e formas de pagamento customizáveis;
  - gastos e entradas;
  - recorrências dos 3 tipos;
  - divisão e acertos;
  - orçamentos;
  - gráficos;
  - tempo real.
- Lighthouse mobile ≥ 90 em Performance e Accessibility na Visão geral.
- Bundle inicial ≤ 250 KB gzip. Recharts carregado sob demanda na Visão geral.
- O `src/web/` antigo é removido no mesmo PR que liga o embed do `web/dist`.
