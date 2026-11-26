# Cartões de crédito: a compra cai no mês da fatura

Data: 2026-11-25. Ideia do irmão do Gustavo, que usa o app.

## Por quê

Uma compra no cartão não sai do bolso no mês em que foi feita, e sim no mês em que a fatura vence. Se a fatura fecha
dia 20 e a compra é no dia 22, ela só é paga no mês seguinte. Hoje o app põe cada lançamento no mês da própria data,
então o mês corrente fica inchado com gastos que só serão pagos depois, e o mês seguinte parece mais folgado do que é.

## Objetivo

- Cadastrar cartões de crédito com dia de fechamento e dia de vencimento.
- Uma compra feita com um cartão conta no **mês do vencimento da fatura** em que ela cai, em **todo o app**: Visão,
  Lançamentos, limites por categoria, essencial/não essencial, mês anterior e assistente.
- Quem olha a lista entende por que uma compra de 22/11 aparece em dezembro.

## Não objetivos (desta versão)

- Parcelamento (uma compra em N parcelas, uma por fatura). É o próximo passo natural; esta versão já guarda o
  vencimento por lançamento, que é a base para ele.
- A fatura como entidade: fechar, pagar, lançamento de "pagamento da fatura", saldo do limite do cartão.
- Mover lançamentos antigos. Os que já existem ficam no mês em que estão.

## Antiobjetivos

- Uma fatura já paga mudar de mês porque o dia de fechamento do cartão foi alterado depois.
- Uma compra antiga pular de mês porque alguém corrigiu só a descrição dela.
- Guardar dados de cartão que importem para fraude. Só os **4 últimos dígitos**, nunca o número inteiro, validade ou CVV.

## Abordagens consideradas

1. **Guardar no lançamento o vencimento da fatura (escolhida).** Calculado uma vez, ao salvar. O mês do lançamento
   vira o mês desse vencimento, e quem não é cartão fica no mês da própria data. O histórico é estável.
2. **Calcular na hora de mostrar** a partir das regras atuais do cartão. Não guarda nada, mas mudar o fechamento
   reescreve o passado, e a conta de datas se repete em umas dez consultas.
3. **Fatura como entidade** (aberta, fechada, paga). Mais completo, umas três vezes maior, e muda a divisão
   (quem pagou a compra ou a fatura?). Dá para chegar nela a partir da opção 1.

O Gustavo pediu a melhor; a escolha foi a 1.

## Regra da fatura

O cartão tem dois números, de 1 a 31:

- **Compras até o dia F** (fechamento): uma compra feita até o dia F, inclusive, entra na fatura que fecha neste dia F.
  O app pede "compras até o dia", que é como o usuário fala, para não haver dúvida sobre o próprio dia do fechamento.
- **Vence no dia V.**

Para uma compra na data `p`:

1. O fechamento é o dia F do mês de `p`, se `p` cair até ele; senão, o dia F do mês seguinte.
2. O vencimento é o primeiro dia V **depois** do fechamento: no mesmo mês, se V > F; senão, no mês seguinte.
3. Um dia maior que o último dia do mês vira o último dia do mês (F = 31 em fevereiro é 28 ou 29), como já acontece
   nas recorrências.

O **melhor dia de compra** é F + 1 (o dia 1, se F for o último dia do mês). O app mostra; não é um campo.

Exemplos que viram teste:

| F  | V  | compra     | fecha      | vence      | mês |
|----|----|------------|------------|------------|-----|
| 20 | 27 | 22/11/2026 | 20/12/2026 | 27/12/2026 | dez |
| 20 | 27 | 20/11/2026 | 20/11/2026 | 27/11/2026 | nov |
| 30 | 7  | 05/11/2026 | 30/11/2026 | 07/12/2026 | dez |
| 31 | 10 | 15/02/2027 | 28/02/2027 | 10/03/2027 | mar |
| 25 | 5  | 26/12/2026 | 25/01/2027 | 05/02/2027 | fev |

## Dados (migração 0008, no lugar, com a cópia de segurança de sempre)

O banco de produção tem dados reais e **não pode ser zerado nem recriado**. A migração só acrescenta colunas
opcionais e um índice; nenhuma linha existente é alterada. Como `invoice_due` nasce vazio, `month` de todo lançamento
antigo é o mês da própria data, ou seja, tudo continua exatamente onde está. Um teste aplica a migração sobre um banco
na versão 7 com lançamentos e confere que os totais de cada mês não mudam.

`payment_methods` ganha colunas opcionais:

- `closing_day INTEGER CHECK (closing_day BETWEEN 1 AND 31)` e `due_day` (mesmo CHECK). Os dois preenchidos significam
  "é cartão de crédito"; os dois vazios, forma de pagamento comum. Um sem o outro é recusado.
- `issuer TEXT` (banco ou bandeira, livre, ex.: "Itaú Mastercard Black"), `card_form TEXT CHECK (card_form IN ('physical','virtual'))`,
  `last4 TEXT CHECK (length(last4) = 4)`. Todos opcionais.

`transactions` ganha:

- `invoice_due TEXT`: o vencimento da fatura (YYYY-MM-DD); vazio quando não foi pago com cartão.
- `month TEXT GENERATED ALWAYS AS (substr(COALESCE(invoice_due, date), 1, 7)) VIRTUAL`, com índice `(wallet_id, month)`.
  Todas as consultas por mês passam a usar `month = ?` em vez de `date BETWEEN`.

Rollback (documentado em `docs/assistente.md`, como as outras): apagar o índice, as colunas `month` e `invoice_due`,
e as colunas novas de `payment_methods`; `PRAGMA user_version=7`.

## Quando o vencimento é calculado

Uma só função Go (sem dependência de banco) calcula o vencimento a partir de (data, F, V), usada por:

- criar lançamento;
- editar lançamento, **só quando a data ou a forma de pagamento mudam**. Corrigir descrição ou valor não move nada.
- o materializador de recorrências, ao gerar a ocorrência do mês;
- a importação de extrato, ao aceitar uma linha de uma conta ligada a um cartão;
- a projeção de recorrências futuras (Próximos e "a pagar"). Uma ocorrência de 22/12 num cartão que fecha dia 20
  aparece em janeiro, então a projeção de um mês olha as ocorrências do mês anterior também.

Mudar o fechamento ou o vencimento do cartão vale para as compras novas ou editadas daí em diante. Transformar a forma
"Crédito" que já existe em cartão também não mexe nas compras antigas.

## O que muda em cada lugar

- **Resumo do mês, mês anterior, por categoria, essencial e Próximos**: filtram por `month`.
  O "realizado" continua sendo data ≤ hoje (ver Questão em aberto).
- **Lista de lançamentos**: a API aceita `?month=YYYY-MM` (o front passa a usar esse parâmetro; `from`/`to` continuam
  para quem precisa de datas). Uma compra cujo mês é diferente do mês da data mostra a data real e uma marca
  "fatura de dez".
- **Divisão (saldo entre membros)**: é um saldo de todo o histórico, não por mês; não muda.
- **Assistente**: totais agrupados por mês usam `month`; períodos por dia ou semana continuam pela data da compra.
  O texto do sistema explica que compras no cartão contam no mês da fatura.
- **Ao salvar** uma compra que cai em outro mês, o aviso diz onde ela foi parar: "Lançado na fatura de dezembro".

## Telas

Cadastro da forma de pagamento (o mesmo `PaymentSheet`), com um interruptor **"Cartão de crédito"**. Ligado, mostra:

```
Cartão de crédito                          [ ● ]
Banco / bandeira          Itaú Mastercard Black
Tipo                         ( Físico | Virtual )
Final                                      1234
Compras até o dia                            20
Vence no dia                                 27
Melhor dia de compra: 21
```

Em Ajustes → Formas de pagamento, o cartão mostra "fecha 20 · vence 27 · final 1234".
Na lista de lançamentos:

```
Restaurante                    − R$ 87,00
22 nov · fatura de dez · Itaú Black
```

## Deixado a quem constrói

Nomes de funções e componentes, onde mora a função do vencimento (pacote próprio pequeno, ex. `internal/billing`),
textos finos de erro, e os testes: a tabela acima, cada consulta por mês com um lançamento de cartão que atravessa o
mês, a edição que não move, a recorrência no cartão, a importação e a migração sobre um banco com dados.

## Questão em aberto

Ver a mensagem de 2026-11-25 ao Gustavo: no mês da fatura, a compra já feita conta como **saiu** ou como **a pagar**
até o vencimento.
