# Pluggy spike: what the API delivers

Date: 2026-10-15. Roadmap: `docs/superpowers/specs/2026-10-09-open-finance-design.md` (phase 4b). The script used (`spikes/pluggy/main.go`) is disposable and lives only on the local branch `spike/pluggy`.

## What was tested

One Pluggy **sandbox** connection ("Pluggy Bank", basic flow), with a checking account and a credit card. No real bank has been connected yet. The script only reads: it authenticates, fetches the item, the accounts and the transactions of the last 90 days, fetches again to check whether the ids repeat, and prints the sign convention.

## Result

| Step | Result |
| --- | --- |
| Authentication (`POST /auth`) | Works. Returns a 2-hour key, in about 200 ms. |
| Item (`GET /items/{id}`) | `status UPDATED`, `execution SUCCESS`. Also returns `consentExpiresAt`, `nextAutoSyncAt` and `webhookUrl`. |
| Accounts (`GET /accounts?itemId=`) | 2 accounts: `BANK/CHECKING_ACCOUNT` (balance R$ 36,286.90) and `CREDIT/CREDIT_CARD` (balance −R$ 838.50). |
| Transactions v2 (`GET /v2/transactions`) | 11 on the checking account and 6 on the card, in ~40 to 50 ms, with cursor pagination. |
| Second fetch | Same ids, same count (`same ids: true`). Importing idempotently by `id` is possible. |

## Findings that change the design

1. **The v2 filters are called `dateFrom` and `dateTo`.** `from` and `to` are rejected with 400. The old endpoint (`GET /transactions`, paged) **already returns 410**: it was turned off before the 2026-12-31 deadline the documentation mentions. Only v2 works.
2. **The sign convention is in the amount, not in `type`.** On the checking account, `DEBIT` comes negative and `CREDIT` comes positive. On the **card**, purchases come as `CREDIT` **with a negative amount**. Safe rule: negative is an expense, positive is an income, and the `type` field is ignored.
3. **Amounts already come with at most 2 decimal places** (no case with more). Conversion to cents is direct, but it must round explicitly.
4. **The credit card bill payment shows up as an expense on the checking account** ("PAGAMENTO FATURA CARTAO VISA", −R$ 167.70) and the card purchases show up again on the card. If both account and card are imported, expenses would count twice. A rule is needed (mark as a transfer or ignore this kind of transaction).
5. **The ids are stable and unique** (11 of 11 and 6 of 6). A unique index on the Pluggy id, like the one statement import already uses, prevents duplicates.
6. **The date comes as date and time**, with a time zone. The import must use the transaction's local date (`America/Sao_Paulo`).
7. **Only `POSTED` transactions appeared.** There were no `PENDING` ones and no installments (`installments: 0`): the sandbox is idealized.

## What the sandbox does not prove

- Whether the **real banks** of Gustavo and Rafa connect and update reliably on the free plan.
- How the real banks send **pending transactions, installments and refunds**.
- The behavior of the **Open Finance consent** (the `consentExpiresAt` field exists: the consent expires and must be renewed by the user).
- The **cost** and the plan limits for continuous use with real accounts.
- How the app would receive **automatic updates** (webhooks require a public URL; the server has one).

## Recommendation

The API fits well with what the app already has: each transaction becomes a row in the **inbox** (the same one used by statement import), with a stable id, signed amount and description. The phase 4b work would be medium-sized.

But **do not build the integration yet.** The sandbox answers whether the API works, not whether it works for your banks. Before writing the plan, a **second round of the spike with a real account** is needed (through Meu Pluggy, `meu.pluggy.ai`), looking specifically at: bank coverage, delay between the purchase and the transaction appearing, pending items, installments, and how long the consent lasts. If that round goes well, the 4b plan starts from findings 1 to 6 above.
