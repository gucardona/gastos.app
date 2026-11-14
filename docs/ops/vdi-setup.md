# Setup único da VDI

Faça uma vez, como root, na VDI. Depois disso o GitHub Actions cuida dos deploys (`.github/workflows/ci.yml`).

**O deploy roda somente em push na `master`.** Branches e pull requests só executam os testes e o build; nunca publicam.

Layout: binários em `/opt/gastos/production/{releases,current}`, dados em `/var/lib/gastos/production`, configuração em `/etc/gastos/production.env`. O app escuta em `127.0.0.1:8000`.

## 1. Usuários e diretórios

```bash
useradd --system --home /var/lib/gastos --shell /usr/sbin/nologin gastos
useradd --create-home --shell /bin/bash deploy

install -d -o deploy -g deploy /opt/gastos/production /opt/gastos/production/releases # o deploy cria o link current dentro de production
install -d -o gastos -g gastos -m 750 /var/lib/gastos/production
install -d /etc/gastos
```

## 2. Configuração

`/etc/gastos/production.env`:

```
GASTOS_ADDR=127.0.0.1:8000
GASTOS_DB_PATH=/var/lib/gastos/production/gastos.db
GASTOS_BACKUP_DIR=/var/lib/gastos/production/backups
```

**Cadastro.** O site é público, então o cadastro é fechado por padrão: só a **primeira conta** é criada sem código; depois disso ninguém consegue se cadastrar. Para convidar outra pessoa, gere um código no servidor e passe-o a ela por onde preferir:

```bash
sudo /opt/gastos/production/current/invite.sh       # vale 24h
sudo /opt/gastos/production/current/invite.sh 2h    # ou outra duração: 30m, 2h, 3d... (de 5m a 7d)
```

Sai uma linha só, por exemplo `Código: K3J9-XQ2M-7PRT (vale até 15/11/2026 às 14:30, uso único)`. O código serve para **um** cadastro e expira no horário indicado; enquanto houver um código válido, a tela de entrada mostra o campo "Código de convite" (maiúsculas, minúsculas, espaços e traços tanto faz). Não é preciso reiniciar o serviço, e o código não fica guardado em lugar nenhum (o banco guarda só um hash). Códigos usados ou vencidos há mais de 7 dias são apagados sozinhos. O script vem com cada deploy e usa o binário da versão no ar.

`GASTOS_INVITE_CODE` não é mais necessário. Ele continua funcionando como alternativa (um código fixo, de pelo menos 8 caracteres, que vale para qualquer número de cadastros enquanto estiver no arquivo, e que exige reiniciar o serviço ao entrar ou sair); se usar, **remova a linha** e reinicie quando todo mundo tiver conta.

**Assistente.** Para habilitar o chat de consulta, acrescente a chave mestra (gere com `openssl rand -base64 32`; o valor fica só no servidor). Sem ela o assistente fica indisponível e o resto funciona normalmente. Detalhes e cuidados em [Assistente de consulta](../assistente.md).

```
GASTOS_SECRET_KEY=<saída do openssl rand -base64 32>
```

`GASTOS_COOKIE_SECURE` fica no padrão (`true`), então o acesso precisa ser por HTTPS (passo 4).

O arquivo guarda segredos (código de convite, chave mestra). Deixe-o legível só pelo root; o systemd o lê como root antes de iniciar o app com o usuário `gastos`, então o app continua recebendo as variáveis:

```bash
chown root:root /etc/gastos/production.env
chmod 600 /etc/gastos/production.env
```

## 3. systemd

```bash
install -m 644 deploy/gastos@.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable gastos@production   # só inicia depois do primeiro deploy
```

Permissão mínima para o usuário de deploy reiniciar só esse serviço, em `/etc/sudoers.d/gastos-deploy` (valide com `visudo -c`):

```
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart gastos@production
```

A unit já vem endurecida: o app só escreve em `/var/lib/gastos/production`, não enxerga a chave nem a cópia do backup fora do servidor (`/var/lib/gastos/.ssh` e `/var/lib/gastos/offsite`), não tem capacidades de root, só abre sockets de rede IPv4/IPv6 e locais e só pode fazer chamadas de sistema comuns de serviços. Nada disso atrapalha o que ele faz: servir HTTP, ler e gravar o SQLite e chamar APIs externas por HTTPS.

### O que o dono do servidor precisa aplicar

O deploy pelo CI troca só o binário; **a unit e as permissões não são atualizadas sozinhas**. Num servidor instalado antes desta versão, rode uma vez, como root, a partir de um clone atualizado do repositório:

```bash
# 1. segredos legíveis só pelo root (o systemd lê o arquivo como root)
chown root:root /etc/gastos/production.env
chmod 600 /etc/gastos/production.env

# 2. unit nova (isolamento do backup e endurecimento)
install -m 644 deploy/gastos@.service /etc/systemd/system/
systemctl daemon-reload
systemctl restart gastos@production

# 3. conferir
systemctl status gastos@production --no-pager
curl -fsS http://127.0.0.1:8000/healthz; echo
systemd-analyze security gastos@production | tail -1   # a nota deve cair em relação à anterior
```

Se o serviço não subir, `journalctl -u gastos@production -n 50 --no-pager` mostra o motivo; para voltar atrás, reinstale a unit anterior (`git log --oneline -- deploy/gastos@.service` mostra o commit; `git show <commit>~1:deploy/gastos@.service` traz o arquivo) e repita `daemon-reload` e `restart`. O serviço de backup fora do servidor não muda.

## 4. HTTPS (Caddy)

O bloco que já existe na VDI não muda: o Caddy repassa `text/event-stream` sem buffer, então o SSE funciona.

```
gastos.gupa.dev {
	reverse_proxy 127.0.0.1:8000
}
```

## 5. Chave SSH do CI

```bash
ssh-keygen -t ed25519 -N '' -f ci_deploy -C gastos-ci
install -d -m 700 -o deploy -g deploy /home/deploy/.ssh
cat ci_deploy.pub >> /home/deploy/.ssh/authorized_keys
ssh-keyscan -t ed25519 <ip-da-vdi>     # saída vira o segredo DEPLOY_KNOWN_HOSTS
```

No GitHub, em Settings → Secrets and variables → Actions, crie:

| Segredo | Valor |
|---|---|
| `DEPLOY_HOST` | IP ou nome da VDI |
| `DEPLOY_SSH_KEY` | conteúdo de `ci_deploy` (privada) |
| `DEPLOY_KNOWN_HOSTS` | saída do `ssh-keyscan` |

Apague `ci_deploy` do servidor depois.

Opcional, mas recomendado: crie a **variável** de repositório (não segredo) `PUBLIC_URL` com o endereço público, por exemplo `https://gastos.gupa.dev`. Depois de cada deploy o CI chama `$PUBLIC_URL/healthz` pelo mesmo caminho dos usuários (DNS, TLS e Caddy), e o job falha se não responder. Sem `DEPLOY_HOST` o job de deploy não faz nada. Para exigir aprovação manual antes de cada deploy, configure revisores obrigatórios no ambiente `production` (Settings → Environments).

## 6. Aposentar o app antigo

O serviço antigo se chama `gastos`, roda `gastos-app` na porta 8000 e usa o banco antigo, que **não é compatível** com o schema novo. Antes do primeiro deploy:

```bash
systemctl disable --now gastos
cp /caminho/do/clone/antigo/gastos.db ~/gastos-antigo.db   # guarde por segurança
```

Depois remova `/etc/systemd/system/gastos.service` e o clone antigo.

## 7. Backup fora do servidor

O backup do app fica no mesmo disco da VDI. Para ter uma cópia cifrada em outro lugar, sem custo, siga [backup.md](backup.md). Faça isso **antes** de usar o app de verdade.

## Primeiro deploy

Faça merge na `master`. O script `scripts/remote-deploy.sh` ativa a release nova e volta sozinho para a anterior se o `/healthz` não responder em 10 segundos.
