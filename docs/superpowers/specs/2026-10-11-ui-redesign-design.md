# Reestruturação da interface: minimalista, estilo Apple

Data: 2026-10-11. Substitui, para a camada visual e de navegação, o desenho de [2026-10-09-frontend-rewrite-design.md](2026-10-09-frontend-rewrite-design.md). A API, os dados e as regras de negócio não mudam.

## Intenção

O app é usado por duas pessoas (a casa: Gustavo e Bia) para lançar gastos rápido e entender para onde o dinheiro foi. A versão atual funciona, mas é confusa: Ajustes aparece em dois lugares, os formulários são pilhas de campos sem hierarquia, o ícone de uma categoria é um campo vazio que pede "um emoji", e os alinhamentos de texto variam de tela em tela.

Sucesso significa:

- **Minimalista e intuitivo, de uso extremamente fácil.** Cada coisa tem um único caminho. Ninguém precisa de instrução para lançar um gasto, aceitar uma linha do extrato ou convidar alguém.
- **Referência visual: produtos da Apple.** Poucos elementos, muito espaço, título grande, listas agrupadas, controles que se explicam.
- **O número é o protagonista.** O que o usuário quer saber (quanto sobrou, para onde foi) é o que mais pesa na tela.

Escolhido pelo usuário entre cinco direções mockadas: cinza frio, blocos brancos arredondados, número grande, barra de navegação flutuante com o "+" separado. As alternativas descartadas (grafite, tipográfica, anel, blocos coloridos) ficam fora de escopo; o modo escuro entra como variante do mesmo sistema, não como direção própria.

## Fora de escopo

Login com Google, Open Finance (Pluggy) e qualquer mudança de API ou de modelo de dados. Eles entram depois, já no sistema novo. Idioma e moeda continuam fixos em pt-BR e real, sem configuração.

## Navegação

Quatro destinos e uma ação:

| Destino | Conteúdo |
|---|---|
| Visão | saldo do mês, para onde foi, recentes |
| Lançamentos | lista do mês por dia, busca, seção "Repetem todo mês" |
| Caixa | linhas de extrato importadas esperando decisão |
| Divisão | acertos entre as pessoas (só quando a carteira divide gastos) |
| + | novo lançamento (folha) |

Regras:

- **Ajustes só pelo avatar** (canto superior direito no celular, rodapé da barra lateral no desktop). Some a engrenagem e a duplicação atual.
- **Recorrentes não é destino.** Vive como seção dentro de Lançamentos e como lista em Ajustes.
- **Carteira ativa** no topo da Visão (nome e "com Bia"); tocar abre a troca de carteira.
- **Celular:** barra flutuante translúcida com quatro abas e o "+" como botão redondo à direita, separado. **A partir de 900 px:** barra lateral de 248 px, com "Novo lançamento" como botão e o avatar embaixo. A aba Divisão só aparece quando a carteira divide gastos.
- A Caixa mostra um contador de pendentes; ele lê o número, sem texto extra.

## Telas

**Visão.** Título do mês ("Outubro") com ‹ › discretos. Abaixo, o saldo em fonte grande, e uma linha com "Entrou" e "Saiu" (pontos coloridos, valores em peso médio). Depois "Para onde foi": categorias com barra fina proporcional, em um bloco agrupado. No desktop, três números lado a lado (saldo, entrou, saiu) e duas colunas (Para onde foi, Recentes).

**Novo lançamento / editar (folha).** Controle segmentado Gasto | Entrada; valor grande no centro; lista agrupada com Categoria, Quando, Pago com, Descrição e o interruptor "Repetir todo mês"; um botão "Salvar" de largura total. Categoria, Quando e Pago com abrem uma escolha visual (grade de categorias, calendário, lista), não um teclado. Em lançamentos com divisão, uma linha "Dividir" abre as partes. Excluir fica no fim da folha de edição, com confirmação.

**Lançamentos.** Título do mês, busca, lista agrupada por dia (data à esquerda, total do dia à direita). Tocar numa linha abre a folha de edição. A seção "Repetem todo mês" lista as recorrências com um interruptor para pausar e a linha abre a edição.

**Caixa.** Topo com "Aceitar N sugeridas" quando houver. Cada linha mostra descrição, valor e conta; tocar abre uma folha com a categoria (grade), o texto da regra opcional ("Usar nas próximas importações"), "Aceitar", "É este lançamento" (quando há candidato) e "Ignorar". Vazia: explica como importar e leva a Ajustes, Contas bancárias.

**Divisão.** Uma frase grande ("Bia deve R$ 120,00 a você") e a ação "Registrar acerto"; histórico abaixo.

**Ajustes.** Duas listas curtas, cada linha com seta e um destino próprio: Perfil; Pessoas, Categorias, Formas de pagamento, Orçamentos; Contas bancárias, Recorrentes, Aparência; Sair (vermelho, no fim). No desktop é mestre e detalhe: a lista à esquerda, o conteúdo à direita.
- **Pessoas:** lista com a permissão como controle segmentado (Pode editar | Só leitura); convidar é um campo de e-mail, o segmentado e um botão "Convidar" de largura total.
- **Categorias:** Gastos | Entradas; "Nova categoria" abre uma folha com pré-visualização (ícone e nome), segmentado de tipo, **cor em uma fileira de amostras** e **ícone em uma grade de emojis com abas por assunto** (Casa, Comida, Transporte, Lazer, Saúde, Dinheiro e outras). Para gastos, o tipo (Essencial | Supérfluo | Investimento) é um segmentado.
- **Contas bancárias:** conta, forma de pagamento, importar extrato (OFX ou CSV, com o mapeamento de colunas só quando CSV) e as regras.
- **Aparência:** Automático | Claro | Escuro.

**Login e cadastro.** Campos grandes, um botão, o alternador de modo como texto. O cadastro mantém "Como você gostaria de ser chamado?" e a confirmação de senha (PR #26).

## Sistema de design

**Cores** (claro): fundo `#F2F3F4`, superfície `#FFFFFF`, tinta `#14181B`, texto secundário `#7B858B`, linha `#ECEEEF`, acento `#0E6577` (só ação e item ativo), entrada `#1E8E5A`, alerta `#C2412D` (estouro de orçamento e ações destrutivas). Gasto não é vermelho: dinheiro que sai usa a tinta normal. O escuro usa os mesmos papéis com fundo `#0F1113`, superfície `#191C1F` e acento `#6CC4D6`, seguindo `prefers-color-scheme`, com o seletor de Aparência para forçar.

**Tipografia:** uma família, Geist (400, 500, 600, 700), números tabulares. Saldo grande 44 px (mobile) e 54 px (desktop) em peso 700 com tracking apertado; títulos de página 32 px; corpo 14 a 15 px; texto secundário 12 a 13 px. Geist é servida do próprio app (arquivo local), sem pedir a terceiros.

**Forma:** blocos agrupados com raio 22, controles com raio 12, botões e seletores em pílula; sem bordas pesadas e sem sombras, exceto a barra flutuante. Espaço generoso: gutter de 20 px no celular, 56 px no desktop.

**Alinhamento:** texto à esquerda, valores à direita, sem centralizar nada além de estados vazios. Isso é uma regra do sistema, não decidida tela a tela.

**Componentes:** `Sheet` (folha com puxador, `<dialog>` nativo), `Segmented`, `GroupedList` e `ListRow` (com seta, valor e variantes), `Switch`, `CategoryGrid`, `EmojiPicker`, `ColorSwatches`, `AmountInput`, `TabBar`, `Sidebar`, `Button` (primário, secundário, destrutivo), `EmptyState`. Os tokens ficam num arquivo CSS único, com variáveis para claro e escuro.

**Movimento:** apenas o que responde a uma ação: folha subindo e descendo, interruptor, troca de aba. Respeita `prefers-reduced-motion`.

**Acessibilidade:** foco visível, alvos de toque de pelo menos 44 px, contraste AA nos dois temas, cada linha de lista como um único controle com nome claro, folha com foco preso e Esc para fechar.

## Entrega

Cinco PRs pequenos, cada um revisável, com CI verde antes do merge. Cada PR reescreve os testes das telas que toca; a API e os dados não mudam.

1. **Base:** tokens, fonte, componentes e a navegação nova (barra flutuante, barra lateral, avatar para Ajustes). As telas antigas continuam funcionando dentro do shell novo.
2. **Visão e Novo lançamento** (folha, seletores visuais).
3. **Lançamentos** (com a seção Repetem todo mês) e **Caixa**.
4. **Ajustes inteiro:** Pessoas, Categorias (cor e emojis), Formas de pagamento, Orçamentos, Contas bancárias, Aparência.
5. **Divisão, login e acabamento:** estados vazios, erros, acessibilidade, passada final de alinhamento.

Cada PR é mergeado na master só com a aprovação do usuário, e o merge dispara o deploy.

## Testes

- **Componentes e telas:** Vitest e Testing Library, por papel e nome acessível (hoje já é assim). Os componentes novos têm testes próprios (segmentado, seletor de emoji, folha: abrir, fechar, foco).
- **e2e (Playwright, desktop e Pixel 5):** os fluxos existentes continuam, com os rótulos novos; ganham um fluxo "criar categoria com cor e emoji", "convidar alguém" e "navegar só pelas quatro abas e pelo avatar".
- **Regressão visual:** capturas de tela das telas principais em claro e escuro, anexadas como artefato da CI, sem comparação automática por enquanto (comparação com tolerância fica para depois).
- **Lint e tipos** continuam obrigatórios.

## Riscos e decisões

- **Escopo grande.** Mitigado pelos cinco PRs: o app continua utilizável a cada merge.
- **Emojis dependem do sistema.** A grade usa emojis do sistema operacional, então a aparência varia entre Windows, Android e iOS. É aceitável; ícones desenhados ficam fora de escopo.
- **Recorrentes fora do menu.** Quem usa muito essa tela precisa de dois toques a mais; em troca, a navegação fica em quatro destinos. Revisitar se isso incomodar.
- **Divisão condicional.** Em carteiras sem divisão, a barra tem três abas mais o "+".
- **Fonte local.** Geist entra no repositório (arquivo de fonte); confirmar a licença (OFL) ao adicionar.
