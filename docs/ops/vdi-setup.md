# Setup único da VDI

Faça uma vez, como root, na VDI. Depois disso o GitHub Actions cuida dos deploys (`.github/workflows/ci.yml`).

Convenções: `production` na porta 8000, `staging` na 8001. Layout em `/opt/gastos/<env>/{releases,current}`, dados em `/var/lib/gastos/<env>`, config em `/etc/gastos/<env>.env`.

## 1. Usuários e diretórios

```bash
useradd --system --home /var/lib/gastos --shell /usr/sbin/nologin gastos
useradd --create-home --shell /bin/bash deploy

for env in production staging; do
  install -d -o deploy -g deploy /opt/gastos/$env/releases
  install -d -o gastos -g gastos -m 750 /var/lib/gastos/$env
done
install -d /etc/gastos
```

## 2. Configuração de cada ambiente

`/etc/gastos/production.env` (no staging: `GASTOS_ADDR=127.0.0.1:8001` e `GASTOS_DB_PATH=/var/lib/gastos/staging/gastos.db`):

```
GASTOS_ADDR=127.0.0.1:8000
GASTOS_DB_PATH=/var/lib/gastos/production/gastos.db
GASTOS_BACKUP_DIR=/var/lib/gastos/production/backups
```

`GASTOS_COOKIE_SECURE` fica no padrão (`true`), então o acesso precisa ser por HTTPS (passo 4).

## 3. systemd

```bash
install -m 644 deploy/gastos@.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable gastos@production gastos@staging   # só inicia depois do primeiro deploy
```

Permissão mínima para o usuário de deploy reiniciar só esses dois serviços, em `/etc/sudoers.d/gastos-deploy` (valide com `visudo -c`):

```
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart gastos@production, /usr/bin/systemctl restart gastos@staging
```

## 4. HTTPS (Caddy)

O bloco de produção que já existe na VDI não muda: o Caddy já repassa `text/event-stream` sem buffer, então o SSE funciona.

```
gastos.gupa.dev {
	reverse_proxy 127.0.0.1:8000
}
```

Para o staging, crie o registro DNS `staging.gastos.gupa.dev` apontando para o IP da VDI e acrescente:

```
staging.gastos.gupa.dev {
	reverse_proxy 127.0.0.1:8001
}
```

Depois `systemctl reload caddy`. Sem o DNS do staging, acesse por túnel (`ssh -L 8001:127.0.0.1:8001 deploy@<ip>`) e use `GASTOS_COOKIE_SECURE=false` no `staging.env`.

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

Apague `ci_deploy` do servidor depois. Sem `DEPLOY_HOST` o job de deploy não faz nada.

## 6. Aposentar o app antigo

O serviço antigo se chama `gastos`, roda `gastos-app` na porta 8000 e usa o banco antigo, que **não é compatível** com o schema novo. Antes do primeiro deploy de produção:

```bash
systemctl disable --now gastos
cp /caminho/do/clone/antigo/gastos.db ~/gastos-antigo.db   # guarde por segurança
```

Depois remova `/etc/systemd/system/gastos.service` e o clone antigo.

## Primeiro deploy

Faça push de uma branch qualquer: ela vai para o staging. O merge em `master` vai para produção. O script `scripts/remote-deploy.sh` volta sozinho para a release anterior se o `/healthz` não responder em 10 segundos.
