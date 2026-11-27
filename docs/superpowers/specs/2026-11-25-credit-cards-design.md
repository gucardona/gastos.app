# Credit cards: a purchase lands in the bill's month

Date: 2026-11-25. Idea from Gustavo's brother, who uses the app.

## Why

A credit card purchase does not leave the pocket in the month it was made, but in the month the bill is due. If the
bill closes on the 20th and the purchase is on the 22nd, it is only paid the following month. Today the app puts each
transaction in the month of its own date, so the current month is inflated with spending that will only be paid
later, and the next month looks lighter than it is.

## Goal

- Register credit cards with a closing day and a due day.
- A purchase made with a card counts in the **month of the due date of the bill** it falls into, in **the whole app**:
  Visão (Overview), Lançamentos (Transactions), category limits, essential/non-essential, previous month and the assistant.
- Anyone looking at the list understands why a purchase made on 11/22 shows up in December.

## Non-goals (this version)

- Installments (one purchase split into N installments, one per bill). This is the natural next step; this version
  already stores the due date per transaction, which is the base for it.
- The bill as an entity: closing, paying, a "bill payment" transaction, card limit balance.
- Moving old transactions. Existing ones stay in the month they are in.

## Anti-goals

- A bill that was already paid changing month because the card's closing day was changed later.
- An old purchase jumping month because someone fixed only its description.
- Storing card data that matters for fraud. Only the **last 4 digits**, never the full number, expiry or CVV.

## Approaches considered

1. **Store the bill's due date on the transaction (chosen).** Computed once, on save. The transaction's month
   becomes the month of that due date, and anything that is not a card stays in the month of its own date. History is stable.
2. **Compute at display time** from the card's current rules. Stores nothing, but changing the closing day
   rewrites the past, and the date math is repeated in about ten queries.
3. **Bill as an entity** (open, closed, paid). More complete, about three times larger, and it changes the split
   (who paid the purchase or the bill?). It can be reached from option 1.

Gustavo asked for the best one; the choice was 1.

## Bill rule

The card has two numbers, from 1 to 31:

- **Purchases up to day F** (closing day): a purchase made up to and including day F goes into the bill that closes on that day F.
  The app asks for "compras até o dia" (purchases up to day), which is how users talk, to avoid doubt about the closing day itself.
- **Due on day V.**

For a purchase on date `p`:

1. The closing date is day F of the month of `p`, if `p` falls on or before it; otherwise, day F of the next month.
2. The due date is the first day V **after** the closing date: in the same month, if V > F; otherwise, in the next month.
3. A day greater than the last day of the month becomes the last day of the month (F = 31 in February is 28 or 29), as
   already happens with recurrences.

The **best purchase day** is F + 1 (day 1, if F is the last day of the month). The app shows it; it is not a field.

Examples that become tests:

| F  | V  | purchase   | closes     | due        | month |
|----|----|------------|------------|------------|-------|
| 20 | 27 | 22/11/2026 | 20/12/2026 | 27/12/2026 | Dec   |
| 20 | 27 | 20/11/2026 | 20/11/2026 | 27/11/2026 | Nov   |
| 30 | 7  | 05/11/2026 | 30/11/2026 | 07/12/2026 | Dec   |
| 31 | 10 | 15/02/2027 | 28/02/2027 | 10/03/2027 | Mar   |
| 25 | 5  | 26/12/2026 | 25/01/2027 | 05/02/2027 | Feb   |

## Data (migration 0008, in place, with the usual backup)

The production database has real data and **cannot be reset or recreated**. The migration only adds optional
columns and an index; no existing row is changed. Since `invoice_due` starts empty, the `month` of every old
transaction is the month of its own date, so everything stays exactly where it is. A test applies the migration to a
version 7 database with transactions and checks that each month's totals do not change.

`payment_methods` gets optional columns:

- `closing_day INTEGER CHECK (closing_day BETWEEN 1 AND 31)` and `due_day` (same CHECK). Both filled in mean
  "is a credit card"; both empty, a regular payment method. One without the other is rejected.
- `issuer TEXT` (bank or card network, free text, e.g. "Itaú Mastercard Black"), `card_form TEXT CHECK (card_form IN ('physical','virtual'))`,
  `last4 TEXT CHECK (length(last4) = 4)`. All optional.

`transactions` gets:

- `invoice_due TEXT`: the bill's due date (YYYY-MM-DD); empty when not paid by card.
- `month TEXT GENERATED ALWAYS AS (substr(COALESCE(invoice_due, date), 1, 7)) VIRTUAL`, with an index `(wallet_id, month)`.
  All queries by month now use `month = ?` instead of `date BETWEEN`.

Rollback (documented in `docs/assistant.md`, like the others): drop the index, the `month` and `invoice_due` columns,
and the new `payment_methods` columns; `PRAGMA user_version=7`.

## When the due date is computed

A single Go function (no database dependency) computes the due date from (date, F, V), used by:

- creating a transaction;
- editing a transaction, **only when the date or the payment method changes**. Fixing the description or amount moves nothing.
- the recurrence materializer, when generating the month's occurrence;
- the bank statement import, when accepting a line from an account linked to a card;
- the projection of future recurrences (Próximos (Upcoming) and "a pagar" (to pay)). An occurrence on 22/12 on a card that closes on the 20th
  shows up in January, so a month's projection also looks at the previous month's occurrences.

Changing the card's closing or due day applies to new or edited purchases from then on. Turning the existing
"Crédito" (Credit) payment method into a card also does not touch old purchases.

## What changes in each place

- **Month summary, previous month, by category, essential and Próximos**: filter by `month`.
  "Realized" is still date ≤ today (see Decisions).
- **Transaction list**: the API accepts `?month=YYYY-MM` (the frontend now uses this parameter; `from`/`to` remain
  for anyone who needs dates). A purchase whose month differs from the month of its date shows the real date and a
  "fatura de dez" (December bill) tag.
- **Divisão (Division) (balance between members)**: it is a balance over the whole history, not per month; it does not change.
- **Assistant**: totals grouped by month use `month`; periods by day or week still use the purchase date.
  The system prompt explains that card purchases count in the bill's month.
- **On save**, a purchase that falls in another month gets a notice saying where it went: "Lançado na fatura de dezembro" (Posted to the December bill).

## Screens

Payment method form (the same `PaymentSheet`), with a **"Cartão de crédito"** (Credit card) switch. When on, it shows:

```
Cartão de crédito                          [ ● ]
Banco / bandeira          Itaú Mastercard Black
Tipo                         ( Físico | Virtual )
Final                                      1234
Compras até o dia                            20
Vence no dia                                 27
Melhor dia de compra: 21
```

In Ajustes → Formas de pagamento (Settings → Payment methods), the card shows "fecha 20 · vence 27 · final 1234".
In the transaction list:

```
Restaurante                    − R$ 87,00
22 nov · fatura de dez · Itaú Black
```

## Left to the implementer

Names of functions and components, where the due date function lives (a small package of its own, e.g. `internal/billing`),
fine-grained error messages, and the tests: the table above, each query by month with a card transaction that crosses the
month, the edit that does not move, the recurrence on the card, the import and the migration on a database with data.

## Decisions

- **A purchase already made counts as "Saiu" (Spent) in the bill's month** (option A, approved by Gustavo on 2026-11-25). Realized
  is still date ≤ today, so category limits catch the purchase on the day it was made. The
  alternative, "a pagar" until the due date, would make the Visão more faithful to the money in the account, but limits would only
  count on the due date, or the app would have two numbers for the same thing.
- The month is the one of the bill's **due date**, and it counts everywhere (Visão, limits, assistant).
