# Backup fora do servidor — plano

**Data:** 2026-10-11
**Status:** script e teste prontos; falta a instalação na VDI (feita por você)

## Problema

O backup rotativo do app fica em `/var/lib/gastos/production/backups`, no mesmo disco do banco. Se o disco da VDI morrer, os dois vão juntos.

## Restrições

Sem custo adicional. O repositório `gastos.app` é **público**, então o backup vai para um repositório **separado e privado** e **cifrado**: o banco tem lançamentos financeiros e hashes de senha.

## Decisão

Um script com `systemd timer` envia, uma vez por dia, o backup mais recente para `gastos-backups` (privado), cifrado com GPG.

- A VDI guarda só a chave **pública**. Quem invadir a VDI não decifra backups antigos. A privada fica com você, protegida por senha.
- Cada envio faz push forçado de **um único commit** com os 14 backups mais novos. O repositório não cresce para sempre, e GitHub Free não cobra repositórios privados.
- O remoto é a fonte da verdade: o script traz o conjunto atual antes de mexer, então recriar a cópia local não perde nada.
- Falha de rede ou de cifragem aborta sem tocar no remoto (um erro de rede não pode virar "recomeçar do zero e apagar tudo").
- A chave de deploy do repositório de backup é **só dele**, com permissão de escrita só nele.

Alternativas descartadas: Litestream e Storage Box (custo ou serviço extra), repositório sem cifrar (dado financeiro legível por quem tiver acesso ao repositório).

## Arquivos

| Arquivo | Papel |
|---|---|
| `scripts/offsite-backup.sh` | cifra e publica o backup mais recente, poda, garante 1 commit |
| `scripts/offsite-backup.test.sh` | teste de ponta a ponta com remoto local e chaves GPG descartáveis |
| `deploy/gastos-offsite-backup.service` / `.timer` | execução diária (04:30, com atraso aleatório e `Persistent`) |
| `docs/ops/backup.md` | instalação, verificação e **restauração** |

## Tarefas

- [x] **1. Script** com checagem do cabeçalho SQLite, cifragem GPG, poda e commit único.
- [x] **2. Teste** (16 verificações, rodado em Ubuntu): primeiro envio, conteúdo cifrado e idêntico ao decifrar, chave pública não decifra, idempotência, rotação, remoto como fonte da verdade, remoto inacessível e destinatário inválido sem tocar no remoto.
- [ ] **3. systemd** (unit e timer) e `docs/ops/backup.md`.
- [ ] **4. CI**: o teste do script roda no job `backend`.
- [ ] **5. Instalação na VDI (manual, depois do merge):** criar o repositório privado, gerar a chave GPG no seu PC, chave de deploy, instalar o script e ativar o timer.
- [ ] **6. Ensaio de restauração** em uma máquina limpa antes de confiar no backup.

## Risco e janela de perda

O app faz backup quando o último tem mais de 24 horas; o envio roda diariamente. O pior caso é perder cerca de um dia e meio de lançamentos. Se isso for pouco, a evolução é um intervalo de backup configurável no app (`GASTOS_BACKUP_EVERY`).

**Perder a chave privada GPG torna todos os backups ilegíveis.** Guarde-a, e a senha, no gerenciador de senhas.
