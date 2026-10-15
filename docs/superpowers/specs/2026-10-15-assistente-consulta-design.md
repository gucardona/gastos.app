# Assistente financeiro, parte 1: base e consulta

Data: 2026-10-15. Estado: aguardando revisão.

## Objetivo

Um consultor financeiro 24h dentro do app. O usuário coloca a própria chave da API da Anthropic em Ajustes e liga o assistente; então aparece uma aba **Chat**, onde ele pergunta em texto sobre os gastos ("quanto gastei nos últimos 15 dias?", "para onde vai a maior parte do dinheiro?") e recebe respostas curtas e diretas, com gráficos simples quando ajudam. O custo do uso é cobrado direto na chave do usuário.

Esta é a **parte 1 de 3**:

1. **Base e consulta (este documento):** chave, configurações, aba Chat, ferramentas somente de leitura.
2. **Escrita com confirmação:** o modelo propõe lançar, editar ou remover, e o usuário confirma num cartão. Inclui decidir em qual carteira um lançamento novo é gravado quando várias estão marcadas.
3. **Voz:** botão de microfone que dita o texto no chat (reconhecimento de fala do navegador, pt-BR).

Fora desta parte: qualquer escrita de dados pelo modelo, voz, Google login, Open Finance.

## Decisões já tomadas

| Tema | Decisão |
| --- | --- |
| Alterações (parte 2) | Toda alteração passa por cartão de confirmação, inclusive lançamentos novos. |
| Histórico | Salvo por usuário e por carteira. Retenção configurável pelo usuário: 1 a 30 dias, padrão 7. |
| Modelo | O usuário escolhe em Ajustes entre Haiku 5.5, Sonnet 5.5 (padrão) e Opus 5.5. |
| Escopo de dados | Seletor de carteiras no chat. Padrão: só a carteira atual. Dropdown com "Selecionar todas" ou marcação individual. Só carteiras de que o usuário é membro. |
| Chave | Uma por usuário, vale em todas as carteiras dele. |
| Abordagem | O servidor roda o laço do modelo com ferramentas de leitura fixas. Sem SQL livre, sem despejar o banco no prompt. |
| Respostas | Concisas e diretas: número primeiro, sem rodeios. Vai no prompt do sistema e reforçado por `max_tokens`. |
| Sugestões na conversa vazia | Não há. A conversa vazia mostra só o campo de texto. |
| Banco | Tudo vai para o `schema.sql` e o mecanismo de migrations é removido. O banco de produção é recriado no deploy (ver Rollout). |

## Dados

Mudanças no `internal/db/schema.sql` (que passa a ser a única definição, incluindo o que hoje está em `migrations/0002_bank_import.sql`):

- `ai_settings` (uma linha por usuário): `user_id` (PK), `api_key_enc` (BLOB, AES-GCM com nonce), `key_hint` (4 últimos caracteres), `model`, `enabled`, `retention_days` (CHECK 1 a 30, padrão 7), `updated_at`.
- `chat_messages`: `id`, `user_id`, `wallet_id` (carteira ativa quando a mensagem foi enviada), `role` (`user`|`assistant`), `content`, `wallet_ids` (JSON das carteiras consultadas), `input_tokens`, `output_tokens`, `created_at`. Índice por `(user_id, wallet_id, id)` e por `created_at`.
- Gráficos: o conteúdo do assistente guarda os cartões de gráfico como JSON junto do texto, para reaparecerem ao reabrir a conversa.

Remoção: `internal/db/migrations/`, as diretivas `//go:embed` das migrations e o laço de `migrate` ficam substituídos por criar o schema na abertura do banco (`CREATE TABLE IF NOT EXISTS`). Os testes de migração (`migration_test.go`) são trocados por um teste de que o schema cria um banco válido e é idempotente.

## Chave da API

- Criptografia: AES-256-GCM. A chave mestra vem de `GASTOS_SECRET_KEY` (32 bytes em base64). Em `internal/config`, a variável é opcional: ausente ou inválida desativa o assistente, nunca impede o servidor de subir; um valor presente porém malformado é erro de configuração.
- A chave da Anthropic nunca é devolvida por nenhuma rota, nunca vai para log nem para mensagem de erro. A API mostra só `key_hint`.
- Ao salvar, o servidor valida a chave com uma chamada mínima à API da Anthropic e responde com erro claro se for recusada. Se a validação falhar por rede, salva como não validada e avisa.

## API

Todas exigem sessão (`user(...)`) e JSON.

- `GET /api/me/ai`: `{available, configured, keyHint, model, enabled, retentionDays}`. `available` é falso sem `GASTOS_SECRET_KEY`.
- `PUT /api/me/ai`: `{apiKey?, model?, enabled?, retentionDays?}`. `apiKey` ausente mantém a atual. `enabled: true` exige chave configurada. Retenção fora de 1 a 30 e modelo fora da lista fixa são erros de validação por campo.
- `DELETE /api/me/ai/key`: remove a chave e desliga o assistente.
- `GET /api/chat?wallet=ID`: mensagens da conversa do usuário nessa carteira (já sem as vencidas).
- `DELETE /api/chat?wallet=ID`: "Nova conversa", apaga as mensagens dela.
- `POST /api/chat`: `{wallet, wallets: [ids], text}`. Resposta em SSE com os eventos `status` (ex.: consultando), `delta` (texto), `chart` (cartão), `done` (uso de tokens) e `error`. Fechar a conexão cancela a chamada à Anthropic.

`/api/me` passa a informar `assistantEnabled` para o frontend decidir se mostra a aba, sem uma segunda chamada.

## Laço do modelo

1. Valida: texto não vazio e até 4000 caracteres, `wallets` não vazio e todas com o usuário como membro (403 caso contrário), assistente ativo e chave presente.
2. Aplica o limite de uso (por usuário, mensagens por minuto, em memória como o limitador de login) e grava a mensagem do usuário.
3. Monta o contexto: instruções do consultor em pt-BR (concisão, números primeiro, valores em R$, sem inventar), data de hoje no fuso configurado, nomes das carteiras marcadas e suas categorias, e as últimas mensagens da conversa até um teto de tamanho.
4. Chama a API de mensagens da Anthropic com as ferramentas, `max_tokens` fixo, em streaming. Se o modelo pedir ferramenta, o servidor executa, devolve o resultado e repete, no máximo 6 voltas por pergunta.
5. Grava a resposta final do assistente com o uso de tokens.

## Ferramentas (somente leitura)

Todas recebem as carteiras marcadas do servidor, nunca do modelo, e validam parâmetros como qualquer rota. Valores em centavos; o modelo formata. Reaproveitam as consultas de `summary.go` em vez de SQL novo.

- `resumo_por_categoria(de, ate)`: total e participação por categoria.
- `totais_por_periodo(de, ate, agrupar)`: `dia`, `semana` ou `mes`.
- `listar_lancamentos(de, ate, categoria?, texto?, limite)`: até 50.
- `maiores_gastos(de, ate, n)`: até 20.
- `mostrar_grafico(tipo, titulo, pontos)`: `barras`, `linha` ou `pizza`, até 24 pontos `{rotulo, valor}`. Não consulta nada; vira o evento `chart`.

Com mais de uma carteira marcada, cada resultado indica a carteira de origem. Descrições de lançamentos são dado, não instrução: nesta parte não existe ferramenta de escrita, então um texto malicioso numa descrição não consegue alterar nada.

## Erros

Chave inválida (401), sem saldo, limite de uso da Anthropic (429), sobrecarga e falha de rede viram o evento `error` com mensagem em pt-BR dizendo o que fazer. Nenhum erro cita a chave. O histórico não se perde.

## Retenção

O `Housekeeping` apaga `chat_messages` mais velhas que `retention_days` de cada usuário (uma consulta com join em `ai_settings`). Usuário sem linha em `ai_settings` não tem mensagens.

## Interface

- **Aba Chat** na navegação (barra inferior no celular, lateral no desktop, depois de Caixa), só quando `assistantEnabled`.
  - Topo: seletor de carteiras (dropdown com caixas de marcar, "Selecionar todas", mínimo uma). Fechado mostra o nome da carteira ou "N carteiras".
  - Mensagens do usuário à direita, do consultor à esquerda, em texto simples. Linha discreta de status enquanto consulta.
  - Rodapé: campo de texto e botão enviar, que vira "parar" durante a resposta. Enter envia, Shift+Enter quebra linha. Menu com "Nova conversa".
  - Conversa vazia: só o campo de texto.
  - Gráficos em SVG simples (barras, linha, pizza), sem biblioteca nova, com as cores dos tokens e texto alternativo ("Alimentação: R$ 820, 31%").
- **Ajustes > Assistente** (grupo "Dados"): chave (campo de senha; salva mostra `•••• 4f2a` com "Trocar" e "Remover"), modelo (Segmented), "Ativar chat", "Guardar conversas por N dias" e o aviso de que os dados consultados são enviados à Anthropic. Sem `GASTOS_SECRET_KEY` no servidor, a tela explica em uma linha em vez de aceitar a chave.
- Segue o sistema visual atual: tokens, contraste, foco e as regras de alinhamento dos testes de estilo.

## Testes (escritos antes do código)

- **Go:** cifra e decifra a chave e nunca a expõe em respostas ou logs; validação de retenção e modelo; salvar recusado sem `GASTOS_SECRET_KEY`; chat contra um servidor de teste que imita a Anthropic (laço de ferramentas, limite de 6 voltas, 401, 429, cancelamento); 403 em carteira alheia; ferramentas somam só as carteiras marcadas; retenção apaga só o vencido de cada usuário; limite de mensagens por minuto; schema criado do zero e idempotente.
- **Vitest:** aba só com assistente ativo, seletor de carteiras (padrão atual, selecionar todas, mínimo uma), envio e parar, cartões de gráfico, tela de Ajustes > Assistente, regras dos testes de estilo.
- **e2e:** configurar, ligar, ver a aba, perguntar e receber resposta com gráfico, contra um servidor Anthropic falso. A chave real nunca entra em teste.

## Rollout

1. PR único da parte 1, com esta spec, o plano e `docs/assistente.md` (pt-BR: configurar a chave, privacidade, custos).
2. Antes do deploy, gerar `GASTOS_SECRET_KEY` na VPS e colocá-la no serviço. O valor fica só no servidor, nunca no chat nem no repositório.
3. Backup do banco e **recriação do banco de produção** (decidido: os dados atuais podem ser descartados). O "sim" é pedido de novo no momento, porque apagar não tem volta.
4. Cada pessoa cria a conta e coloca a própria chave em Ajustes. O Rafa usa a chave dele.

## Riscos conhecidos

- A chave mestra perdida ou trocada invalida as chaves salvas; o usuário precisa colocá-las de novo. Documentado.
- Os dados dos lançamentos consultados vão para a Anthropic; o aviso fica visível em Ajustes.
- O custo de uma conversa longa cresce com o contexto; o teto de histórico enviado e o `max_tokens` limitam isso.
