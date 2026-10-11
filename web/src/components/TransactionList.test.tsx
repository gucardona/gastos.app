import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { toISODate, today } from "../lib/dates";
import { appCtx, renderApp, tx } from "../test/fixtures";
import { TransactionList } from "./TransactionList";

const yesterday = () => {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  return toISODate(d);
};

describe("TransactionList", () => {
  it("agrupa por dia com o total do dia e sinais corretos", () => {
    renderApp(
      <TransactionList
        items={[
          tx({ id: 1, amountCents: 5000, date: today() }),
          tx({ id: 2, kind: "income", categoryId: 3, amountCents: 100000, description: "Salário", date: today() }),
          tx({ id: 3, amountCents: 2000, description: "Padaria", date: yesterday() }),
        ]}
      />,
    );
    const hoje = screen.getByText(/^Hoje,/).closest("section")!;
    expect(within(hoje).getByText(/\+R\$\s950,00/)).toBeTruthy(); // 1000 − 50 no cabeçalho do dia
    expect(within(hoje).getByText(/−R\$\s50,00/)).toBeTruthy();
    expect(within(hoje).getByText(/\+R\$\s1\.000,00/)).toBeTruthy();
    expect(screen.getByText(/^Ontem,/)).toBeTruthy();
  });

  it("mostra categoria, pagamento e o selo de recorrente", () => {
    renderApp(<TransactionList items={[tx({ recurrenceId: 7 })]} />);
    expect(screen.getByText(/Mercado, Pix/)).toBeTruthy();
    expect(screen.getByText("recorrente")).toBeTruthy();
  });

  it("explica quem pagou e com quem foi dividido", () => {
    renderApp(
      <TransactionList
        items={[tx({ paidByUserId: 2, splits: [{ userId: 1, shareBp: 5000, amountCents: 2445 }, { userId: 2, shareBp: 5000, amountCents: 2445 }] })]}
      />,
    );
    expect(screen.getByText(/Bia pagou, dividido com Ana/)).toBeTruthy();
  });

  it("abre a edição ao tocar na linha", async () => {
    const ctx = appCtx();
    const t = tx();
    renderApp(<TransactionList items={[t]} />, ctx);
    await userEvent.click(screen.getByRole("button", { name: /Mercado do bairro/ }));
    expect(ctx.openAdd).toHaveBeenCalledWith(t);
  });

  it("leitores não editam: a linha fica desabilitada", async () => {
    const ctx = appCtx({ canEdit: false });
    renderApp(<TransactionList items={[tx()]} />, ctx);
    const row = screen.getByRole("button", { name: /Mercado do bairro/ }) as HTMLButtonElement;
    expect(row.disabled).toBe(true);
  });
});
