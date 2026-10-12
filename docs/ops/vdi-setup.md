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

**Cadastro.** O site é público, então o cadastro é fechado por padrão: só a **primeira conta** é criada sem código; depois disso ninguém consegue se cadastrar. Para convidar outra pessoa (a Bia, por exemplo), acrescente uma linha ao arquivo, reinicie o serviço e passe o código a ela por onde preferir:

```
GASTOS_INVITE_CODE=<um segredo de pelo menos 8 caracteres>
```

Quando todo mundo já tiver conta, **remova a linha** e reinicie para fechar o cadastro de novo. Quem já tem conta não é afetado.

`GASTOS_COOKIE_SECURE` fica no padrão (`true`), então o acesso precisa ser por HTTPS (passo 4).

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
