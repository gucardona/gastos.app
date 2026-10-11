# Backup fora do servidor

O app já faz um backup diário no disco da VDI (`/var/lib/gastos/production/backups`, os 14 mais recentes). Isso não protege de perder o disco. Este guia manda uma cópia **cifrada** para um repositório **privado** no GitHub, sem custo.

Como funciona: um timer do systemd roda `scripts/offsite-backup.sh` todo dia às 04:30. O script cifra o backup mais recente com a chave **pública** GPG, mantém os 14 mais novos e publica um único commit (push forçado), então o repositório não cresce. A VDI nunca tem a chave privada: só você consegue decifrar.

> **Guarde a chave privada GPG e a senha dela no seu gerenciador de senhas.** Sem elas os backups viram lixo ilegível, e ninguém consegue recuperá-los.

O repositório `gastos.app` é público. **Nunca** coloque o backup nele: use um repositório separado e privado.

## 1. Instalar uma vez

### No seu PC (Git Bash)

```bash
# chave GPG: pede uma senha; a pública vai para a VDI, a privada fica com você
gpg --quick-generate-key "gastos-backup" default default never
gpg --armor --export gastos-backup > gastos-backup.pub.asc
gpg --armor --export-secret-keys gastos-backup > gastos-backup.secret.asc   # guarde no gerenciador de senhas e apague o arquivo
gpg --fingerprint gastos-backup                                               # anote o fingerprint

# repositório privado (precisa ser --private)
gh repo create gastos-backups --private --description "Backups cifrados do gastos"
```

### Na VDI (como root)

```bash
# chave de acesso só para esse repositório
install -d -m 700 -o gastos -g gastos /var/lib/gastos/.ssh
sudo -u gastos ssh-keygen -t ed25519 -N '' -f /var/lib/gastos/.ssh/backup_key -C gastos-offsite
ssh-keyscan -t ed25519 github.com > /var/lib/gastos/.ssh/known_hosts
chown gastos:gastos /var/lib/gastos/.ssh/known_hosts
cat /var/lib/gastos/.ssh/backup_key.pub     # cadastre no GitHub, abaixo

# a chave pública do GPG (copie o gastos-backup.pub.asc do seu PC)
sudo -u gastos gpg --import gastos-backup.pub.asc

# script, serviço e timer
install -m 755 scripts/offsite-backup.sh /usr/local/bin/gastos-offsite-backup
install -m 644 deploy/gastos-offsite-backup.service deploy/gastos-offsite-backup.timer /etc/systemd/system/
```

No seu PC, cadastre a chave de acesso **com permissão de escrita, só no repositório de backup**:

```bash
gh repo deploy-key add backup_key.pub -R gucardona/gastos-backups --title vdi --allow-write
```

De volta à VDI, crie `/etc/gastos/offsite.env` (permissão 640, dono `root:gastos`):

```
BACKUP_REPO=git@github.com:gucardona/gastos-backups.git
GPG_RECIPIENT=<fingerprint anotado acima>
GIT_SSH_COMMAND="ssh -i /var/lib/gastos/.ssh/backup_key -o IdentitiesOnly=yes -o UserKnownHostsFile=/var/lib/gastos/.ssh/known_hosts -o StrictHostKeyChecking=yes"
```

Ative:

```bash
systemctl daemon-reload
systemctl enable --now gastos-offsite-backup.timer
systemctl start gastos-offsite-backup.service   # primeiro envio agora
journalctl -u gastos-offsite-backup -n 20 --no-pager
```

A última linha do log deve ser `enviado: gastos-AAAAMMDD-HHMMSS.db.gpg (1 arquivos no repositório)`.

## 2. Conferir de vez em quando

```bash
systemctl list-timers gastos-offsite-backup.timer    # próxima execução
journalctl -u gastos-offsite-backup --since "2 days ago" --no-pager
```

No GitHub, o repositório `gastos-backups` deve ter **um commit** recente e até 14 arquivos `.gpg`. Se o serviço falhar, `systemctl status gastos-offsite-backup` mostra por quê (rede, chave de acesso removida, GPG sem a chave pública).

## 3. Restaurar

Faça isto no seu PC (que tem a chave privada) ou em qualquer máquina onde você importe `gastos-backup.secret.asc`.

```bash
git clone git@github.com:gucardona/gastos-backups.git
cd gastos-backups
ls                                                        # escolha o arquivo pelo nome (data e hora)
gpg --decrypt gastos-20261011-043000.db.gpg > gastos.db   # pede a senha da chave
```

Na VDI, com o app parado:

```bash
systemctl stop gastos@production
cd /var/lib/gastos/production
mv gastos.db gastos.db.antes              # não apague antes de conferir
rm -f gastos.db-wal gastos.db-shm
install -m 640 -o gastos -g gastos /caminho/gastos.db gastos.db
systemctl start gastos@production
curl -fsS http://127.0.0.1:8000/healthz
```

Entre no app e confira os últimos lançamentos. Só depois remova `gastos.db.antes`.

**Ensaie a restauração uma vez**, em outra máquina ou numa pasta de teste, antes de precisar dela de verdade.

## 4. Quanto dado pode se perder

O app faz backup quando o último tem mais de 24 horas e o envio roda uma vez por dia. No pior caso você perde cerca de um dia e meio de lançamentos. Se isso for muito, o caminho é um intervalo de backup configurável no app.

## 5. Trocar a chave

Gere uma chave nova, importe a pública na VDI, troque `GPG_RECIPIENT` e reinicie o timer. Os backups antigos continuam cifrados com a chave antiga: **guarde a chave antiga** enquanto eles forem a única cópia.
