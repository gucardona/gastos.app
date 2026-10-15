# Assistente de consulta

Um chat dentro do app para perguntar sobre os seus próprios lançamentos ("quanto gastei este mês?", "quais foram meus maiores gastos?"). O assistente só **consulta**: não cria, altera nem apaga nada. Ele usa a API da Anthropic (Claude), com a chave de cada pessoa, e pode responder com pequenos gráficos.

## Como funciona

- Cada pessoa coloca a **própria chave** da API da Anthropic em **Ajustes > Assistente**, escolhe o modelo (Haiku, Sonnet ou Opus) e liga **Ativar chat**. A aba **Chat** só aparece depois disso.
- No Chat, o seletor no topo define quais carteiras são consultadas (por padrão, a carteira atual; "Selecionar todas" inclui as outras). Só entram carteiras de que você participa.
- A conversa fica guardada por alguns dias (padrão 7, de 1 a 30, em Ajustes > Assistente). **Nova conversa** apaga o histórico daquela carteira.

## Configuração no servidor

O servidor guarda as chaves cifradas (AES-256-GCM) com uma **chave mestra** vinda da variável `GASTOS_SECRET_KEY`: 32 bytes em base64.

```bash
openssl rand -base64 32
```

Coloque o resultado em `/etc/gastos/production.env` (o mesmo arquivo do serviço na VPS) e reinicie o serviço:

```
GASTOS_SECRET_KEY=<saída do comando acima>
```

- O valor fica só no servidor: nunca no repositório, no chat ou em log.
- Sem a variável, o servidor sobe normalmente e o assistente aparece como indisponível. Com a variável presente mas malformada, o servidor **não sobe** (erro de configuração).
- Faça uma cópia da chave mestra em local seguro (gerenciador de senhas).
- Se a chave mestra **mudar** (ou for perdida), as chaves guardadas deixam de poder ser lidas: o chat pede para informar a chave de novo e Ajustes mostra a chave como não configurada. Nada mais é perdido; cada pessoa só precisa colar a própria chave outra vez.
- `GASTOS_ANTHROPIC_URL` (padrão `https://api.anthropic.com`) existe só para testes automatizados; não defina em produção.

## Privacidade

Ao perguntar, o servidor envia à Anthropic a sua pergunta, a conversa recente e os resultados das consultas feitas (por exemplo, descrições, categorias e valores dos lançamentos das carteiras selecionadas). Isso acontece com a sua chave e está sujeito às regras da Anthropic. A chave nunca volta ao navegador (só os 4 últimos caracteres) e não aparece em logs. Se não quer que determinada carteira seja consultada, desmarque-a no seletor.

## Custos

O uso é cobrado na conta da Anthropic dona de cada chave, não pelo app. Modelos maiores (Opus) custam mais que os menores (Haiku). O modelo e a retenção da conversa se ajustam em Ajustes > Assistente. Há um limite de mensagens por minuto por pessoa.

## Roteiro de deploy (versão com assistente)

Esta versão cria um banco novo. O banco atual **não é migrado**: ele é movido para fora do caminho e cada pessoa recria a própria conta. Faça só com o "sim" de quem usa o app, porque os dados atuais deixam de aparecer.

1. Gere a chave mestra na VPS (`openssl rand -base64 32`) e coloque `GASTOS_SECRET_KEY` em `/etc/gastos/production.env`.
2. **Backup** do banco atual e da pasta de backups (veja [docs/ops/backup.md](ops/backup.md)); confira que a cópia abre.
3. **Pare** o serviço: `systemctl stop gastos@production`.
4. **Mova** o banco antigo para fora do caminho, por exemplo `mv /var/lib/gastos/production/gastos.db{,-wal,-shm} /var/lib/gastos/antigo/` (crie a pasta antes; mova também os arquivos `-wal` e `-shm`, se existirem).
5. **Suba** a nova versão (deploy normal pela `master`) e inicie o serviço: o app cria o banco novo e vazio.
6. Cada pessoa **recria a conta** (a primeira entra sem código; as outras com `GASTOS_INVITE_CODE`) e depois coloca a própria chave em Ajustes > Assistente.
7. Quando todos tiverem conta, remova `GASTOS_INVITE_CODE` e reinicie, como no setup da VDI.
