# Importing the bank statement

You download the statement from your bank, upload it to gastos and decide, line by line, what becomes a transaction. Nothing is recorded without your confirmation.

## Step by step

1. **Download the statement** in the bank's app or internet banking. Look for "exportar extrato" (export statement) or "baixar extrato" (download statement) and choose **OFX** (also called "Money" or "Quicken"). If your bank only offers CSV, that works too. Each bank names and organizes this differently, so there is no single path.
2. In **Ajustes, Contas bancárias** (Settings, Bank accounts), create an account (for example "Nubank cartão") and, if you want, choose the payment method its transactions will use.
3. Click **Importar extrato** (Import statement), then choose the format and the file.
4. Open the **Caixa de entrada** (Inbox) and, for each line, choose the category and click **Aceitar** (Accept), or **Ignorar** (Ignore) what you do not care about.

Accepted transactions are created with the statement's date and amount, and the payer is whoever is set on the bank account.

## Supported formats

- **OFX** (recommended): includes date, amount and a unique identifier per line. Files up to 5 MB.
- **CSV**: the separators `;`, `,` or tab are detected automatically. Values such as `1.234,56` and `1,234.56` are understood. Dates in `dd/mm/aaaa` or `aaaa-mm-dd` (day/month/year or year-month-day). You give the column number (starting at 1) for date, description and amount, and whether the first line is a header.
  - In a credit card bill, purchases usually appear as **positive** values. Check "Compras aparecem como valores positivos" (Purchases appear as positive values) so gastos treats them as expenses.

A negative value becomes an expense and a positive value becomes an income. Lines with a zero value are skipped.

## Importing again is safe

- Uploading the same file again, or a statement that overlaps another one, does not duplicate anything: lines already seen are counted as "já importadas" (already imported).
- A line you ignored, or whose transaction you deleted, does not come back to the inbox.
- CSV has no identifier per line, so two identical purchases on the same day stay distinct, but editing the file by hand and uploading it again can bring lines back.

## Category rules

When you accept a line without a suggested category, check "Usar esta categoria nas próximas importações" (Use this category in future imports) and confirm the text (for example `uber`). In later imports, lines whose description contains that text (case and accent insensitive) arrive with the chosen category already set. The rules are in Ajustes, Contas bancárias, and can be deleted.

An expense rule is never applied to an income, and vice versa.

## Transactions you already entered by hand

If there is a transaction of the same type and amount, with a date up to two days apart, the line shows "Parece o lançamento que você já fez" (Looks like the transaction you already entered). Use **É este lançamento** (This is the transaction) to link the two without creating a second one. These cases are left out of "Aceitar sugeridas" (Accept suggested), precisely so you can decide.

## Limitations

- OFX files vary between banks. The parser is tolerant, but it was only tested with the standard format; if a file is rejected, open an issue with an anonymized excerpt.
- There is no automatic connection to the bank (Open Finance). Importing is always manual.
- Only people who can edit the wallet can import; readers see the inbox but do not decide.
