# Spike da Pluggy: o que a API entrega

Data: 2026-10-15. Roteiro: `docs/superpowers/specs/2026-10-09-open-finance-design.md` (fase 4b). O script usado (`spikes/pluggy/main.go`) é descartável e ficou só na branch local `spike/pluggy`.

## O que foi testado

Uma conexão do **sandbox** da Pluggy ("Pluggy Bank", fluxo básico), com uma conta corrente e um cartão de crédito. Nenhum banco real foi conectado ainda. O script só lê: autentica, busca o item, as contas e as transações dos últimos 90 dias, busca de novo para checar se os ids se repetem e imprime a convenção de sinais.

## Resultado

| Passo | Resultado |
| --- | --- |
| Autenticação (`POST /auth`) | Funciona. Devolve uma chave de 2 horas, em cerca de 200 ms. |
| Item (`GET /items/{id}`) | `status UPDATED`, `execution SUCCESS`. Traz também `consentExpiresAt`, `nextAutoSyncAt` e `webhookUrl`. |
| Contas (`GET /accounts?itemId=`) | 2 contas: `BANK/CHECKING_ACCOUNT` (saldo R$ 36.286,90) e `CREDIT/CREDIT_CARD` (saldo −R$ 838,50). |
| Transações v2 (`GET /v2/transactions`) | 11 na conta corrente e 6 no cartão, em ~40 a 50 ms, com paginação por cursor. |
| Segunda busca | Mesmos ids, na mesma quantidade (`same ids: true`). Dá para importar de forma idempotente pelo `id`. |

## Descobertas que mudam o desenho

1. **Os filtros do v2 se chamam `dateFrom` e `dateTo`.** `from` e `to` são recusados com 400. O endpoint antigo (`GET /transactions`, por página) **já responde 410**: foi desligado antes do prazo de 31/12/2026 que a documentação cita. Só o v2 vale.
2. **A convenção de sinais é pelo valor, não pelo `type`.** Na conta corrente, `DEBIT` vem negativo e `CREDIT` vem positivo. No **cartão**, as compras vêm como `CREDIT` **com valor negativo**. Regra segura: negativo é despesa, positivo é receita, e o campo `type` é ignorado.
3. **Os valores já vêm sem mais de 2 casas decimais** (nenhum caso com mais). A conversão para centavos é direta, mas deve arredondar de forma explícita.
4. **O pagamento da fatura aparece como despesa na conta corrente** ("PAGAMENTO FATURA CARTAO VISA", −R$ 167,70) e as compras do cartão aparecem de novo no cartão. Importando conta e cartão, os gastos contariam duas vezes. Precisa de uma regra (marcar como transferência ou ignorar esse tipo de lançamento).
5. **Os ids são estáveis e únicos** (11 de 11 e 6 de 6). Um índice único no id da Pluggy, igual ao que a importação de extrato já usa, evita duplicar.
6. **A data vem como data e hora**, com fuso. A importação deve usar a data local (`America/Sao_Paulo`) do lançamento.
7. **Só apareceram transações `POSTED`.** Não houve `PENDING` nem parcelamentos (`installments: 0`): o sandbox é idealizado.

## O que o sandbox não prova

- Se os **bancos reais** do Gustavo e da Rafa se conectam e atualizam com estabilidade pelo plano gratuito.
- Como os bancos reais mandam **lançamentos pendentes, parcelas e estornos**.
- O comportamento do **consentimento do Open Finance** (o campo `consentExpiresAt` existe: o consentimento vence e precisa ser renovado pelo usuário).
- O **custo** e os limites do plano para uso contínuo com contas reais.
- Como o app receberia **atualizações automáticas** (webhooks exigem uma URL pública; o servidor tem uma).

## Recomendação

A API se encaixa bem no que o app já tem: cada transação vira uma linha na **caixa de entrada** (a mesma da importação de extrato), com id estável, valor assinado e descrição. O trabalho da fase 4b seria de tamanho médio.

Mas **não construir a integração ainda.** O sandbox responde se a API funciona, não se ela serve para os bancos de vocês. Antes de escrever o plano, falta uma **segunda rodada do spike com uma conta real** (pelo Meu Pluggy, `meu.pluggy.ai`), olhando especificamente: cobertura do banco, atraso entre a compra e o lançamento, pendentes, parcelas, e quanto tempo o consentimento dura. Se essa rodada der certo, o plano da 4b parte do que está nas descobertas 1 a 6 acima.
