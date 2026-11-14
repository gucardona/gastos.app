import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Summary, Transaction } from "../api/types";
import { addMonths, monthLabel, monthRange, today } from "../lib/dates";
import { appCtx, bodyOf, mockFetch, renderApp, tx, wallet } from "../test/fixtures";
import { Overview } from "./Overview";

const summary = (over: Partial<Summary> = {}): Summary => ({
  month: today().slice(0, 7), incomeCents: 850000, expenseCents: 412340, balanceCents: 437660, dailyAverageCents: 37485,
  byCategory: [
    { categoryId: 1, kind: "expense", cents: 104230, budgetCents: 150000 },
    { categoryId: 2, kind: "expense", cents: 64000, budgetCents: 50000 },
    { categoryId: 3, kind: "income", cents: 850000, budgetCents: null },
  ],
  byEssentiality: {}, previous: { incomeCents: 0, expenseCents: 398000 }, upcomingIncomeCents: 0, upcomingExpenseCents: 0, upcoming: [], ...over,
});

function serve(s: Summary, txs: Transaction[]) {
  return mockFetch((url, init) => {
    if (init.method && init.method !== "GET") return { body: {} };
    return { body: url.includes("/summary") ? s : txs };
  });
}
const writes = (fetch: ReturnType<typeof mockFetch>) => fetch.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const ticks = (container: HTMLElement) => Array.from(container.querySelectorAll(".ruler i"));
const daysIn = (month: string) => Number(monthRange(month).to.slice(8));

describe("Overview: numbers", () => {
  it("splits reais and cents in the balance and shows income and expenses", async () => {
    serve(summary(), [tx()]);
    renderApp(<Overview />);
    const balance = await screen.findByText("4.376");
    expect(balance.closest(".balance")!.textContent).toBe("R$4.376,60");
    expect(screen.getByText(/Entrou/).textContent).toMatch(/8\.500,00/);
    expect(screen.getByText(/Saiu/).textContent).toMatch(/4\.123,40/);
    expect(balance.closest(".balance")!.className).not.toContain("negative");
  });

  it("highlights a negative balance", async () => {
    serve(summary({ balanceCents: -4890, incomeCents: 0, expenseCents: 4890 }), [tx()]);
    renderApp(<Overview />);
    const balance = await screen.findByText("-48");
    expect(balance.closest(".balance")!.className).toContain("negative");
  });

  it("shows the daily average and the previous month's close", async () => {
    serve(summary(), [tx()]);
    renderApp(<Overview />);
    expect(await screen.findByText(/Média de/)).toBeTruthy();
    expect(screen.getByText(/R\$\s374,85/)).toBeTruthy();
    expect(screen.getByText(/mês anterior fechou em/)).toBeTruthy();
    expect(screen.getByText(/R\$\s3\.980,00/)).toBeTruthy();
  });

  it("with no average or previous month, it does not write an empty sentence", async () => {
    serve(summary({ dailyAverageCents: 0, previous: { incomeCents: 0, expenseCents: 0 } }), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("4.376");
    expect(container.querySelector(".strip-note")!.textContent).toBe("");
  });
});

describe("Overview: states", () => {
  it("invites the user to add something when the month is empty", async () => {
    serve(summary({ byCategory: [], balanceCents: 0, incomeCents: 0, expenseCents: 0 }), []);
    renderApp(<Overview />);
    expect(await screen.findByText(/Nada lançado em Casa neste mês/)).toBeTruthy();
    expect(document.querySelector(".ruler")).toBeNull();
  });

  it("shows loading while the API has not answered", () => {
    mockFetch(() => new Promise(() => {}) as never);
    const { container } = renderApp(<Overview />);
    expect(container.querySelectorAll(".skeleton").length).toBeGreaterThan(0);
    expect(screen.queryByText("Saldo do mês")).toBeNull();
  });

  it("shows the error with a retry option", async () => {
    const fetch = mockFetch({ status: 500, body: { error: { code: "internal", message: "Erro interno" } } });
    const user = userEvent.setup();
    renderApp(<Overview />);
    expect(await screen.findByText("Não foi possível carregar este mês.")).toBeTruthy();
    const before = fetch.mock.calls.length;
    await user.click(screen.getByRole("button", { name: "Tentar de novo" }));
    await waitFor(() => expect(fetch.mock.calls.length).toBeGreaterThan(before));
  });

  it("only offers 'Ver todos' when there are more transactions than the list shows", async () => {
    const many = Array.from({ length: 8 }, (_, i) => tx({ id: i + 1, description: `Item ${i + 1}` }));
    serve(summary(), many);
    renderApp(<Overview />);
    await screen.findByText("Item 1");
    expect(screen.queryByText("Item 7")).toBeNull(); // only the first 6
    expect(screen.getByRole("link", { name: "Ver todos os lançamentos" }).getAttribute("href")).toMatch(/^\/lancamentos\?m=\d{4}-\d{2}$/);
  });

  it("without extra transactions, it does not show the link", async () => {
    serve(summary(), [tx()]);
    renderApp(<Overview />);
    await screen.findByText("Mercado do bairro");
    expect(screen.queryByRole("link", { name: "Ver todos os lançamentos" })).toBeNull();
  });
});

describe("Overview: month ruler", () => {
  const current = today().slice(0, 7);
  const todayDay = Number(today().slice(8));

  it("marks today, dots the future and gives height to days with spending", async () => {
    serve(summary(), [tx({ amountCents: 20000, date: today() })]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("4.376");
    const all = ticks(container);
    expect(all).toHaveLength(daysIn(current));
    expect(all.filter((t) => t.className === "future")).toHaveLength(daysIn(current) - todayDay);
    const hoje = all[todayDay - 1] as HTMLElement;
    expect(hoje.className).toBe("today");
    expect(hoje.style.height).toBe("72px"); // the largest expense fills the whole ruler
    expect(hoje.getAttribute("title")).toMatch(/R\$\s200,00/);
    if (todayDay > 1) expect(all[0].className).toBe("zero"); // previous day with no spending
  });

  it("says in text, for screen readers, what each day with spending cost", async () => {
    serve(summary(), [tx({ amountCents: 20000, date: today() })]);
    renderApp(<Overview />);
    await screen.findByText("4.376");
    const ruler = screen.getByRole("region", { name: "Gastos por dia" });
    const text = ruler.querySelector(".visually-hidden")?.textContent ?? "";
    expect(text).toMatch(new RegExp(`Dia ${todayDay}: R\\$\\s200,00`));
    expect(text).not.toMatch(/sem gastos/);
  });

  it("the recent lines are grouped under day headings one level below 'Recentes'", async () => {
    serve(summary(), [tx({ amountCents: 20000, date: today() })]);
    renderApp(<Overview />);
    await screen.findByText("4.376");
    expect(screen.getByRole("heading", { level: 3, name: /^Hoje,/ })).toBeTruthy();
  });

  it("height is proportional (square root) to the month's largest expense", async () => {
    if (todayDay < 2) return; // needs two days in the month to compare
    const yesterday = today().slice(0, 8) + String(todayDay - 1).padStart(2, "0");
    serve(summary(), [tx({ id: 1, amountCents: 40000, date: today() }), tx({ id: 2, amountCents: 10000, date: yesterday })]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("4.376");
    const all = ticks(container) as HTMLElement[];
    expect(all[todayDay - 1].style.height).toBe("72px");
    expect(all[todayDay - 2].style.height).toBe("36px"); // sqrt(10000/40000) = 0,5
  });

  it("ignores income on the ruler (expenses only)", async () => {
    serve(summary(), [tx({ kind: "income", categoryId: 3, amountCents: 99999, date: today() })]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("4.376");
    // today with no spending stays a thin tick (today's mark on the axis remains); income does not become a bar
    expect(ticks(container)[todayDay - 1].className).toBe("zero");
    expect((ticks(container)[todayDay - 1] as HTMLElement).style.height).toBe("");
  });

  it("past month: no day is future or 'today'", async () => {
    const past = addMonths(current, -1);
    serve(summary({ month: past }), [tx({ date: `${past}-10` })]);
    const { container } = renderApp(<Overview />, appCtx({ month: past }));
    await screen.findByText("4.376");
    const all = ticks(container);
    expect(all).toHaveLength(daysIn(past));
    expect(all.filter((t) => t.className === "future" || t.className === "today")).toHaveLength(0);
  });

  it("future month: every day is dotted", async () => {
    const next = addMonths(current, 1);
    serve(summary({ month: next }), [tx({ date: `${next}-10` })]);
    const { container } = renderApp(<Overview />, appCtx({ month: next }));
    await screen.findByText("4.376");
    expect(ticks(container).every((t) => t.className === "future")).toBe(true);
  });

  it("the month title does not depend on the day (label sanity check)", () => {
    expect(monthLabel("2026-10")).toBe("outubro de 2026");
  });
});

describe("Overview: budget rails", () => {
  it("lists only expense categories, with the limit when there is one", async () => {
    serve(summary(), [tx()]);
    renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    expect(screen.getAllByText("Mercado").length).toBeGreaterThan(0); // the pie legend repeats the names
    expect(screen.getAllByText("Restaurante").length).toBeGreaterThan(0);
    expect(screen.queryByText("Salário")).toBeNull(); // income is excluded
    expect(screen.getByText(/de R\$\s1\.500,00/)).toBeTruthy();
  });

  it("flags the overspend with the difference and the alert color", async () => {
    serve(summary(), [tx()]);
    const { container } = renderApp(<Overview />);
    expect(await screen.findByText(/R\$\s140,00 acima do orçamento/)).toBeTruthy();
    const over = container.querySelectorAll(".track.over");
    expect(over).toHaveLength(1);
    expect((over[0].querySelector(".limit") as HTMLElement).style.left).toBe("calc(78.125% - 1px)"); // scale = the larger of spent and limit: 50000 / 64000
  });

  it("within budget: bar proportional to the limit and no alert", async () => {
    serve(summary(), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    const mercado = container.querySelectorAll(".rail")[0];
    expect(mercado.querySelector(".track")!.classList.contains("over")).toBe(false);
    expect((mercado.querySelector(".fill") as HTMLElement).style.width).toMatch(/^69\.4/); // 104230 / 150000
    expect(mercado.querySelector(".warn")).toBeNull();
  });

  it("with no budget, it draws no limit mark", async () => {
    serve(summary({ byCategory: [{ categoryId: 1, kind: "expense", cents: 5000, budgetCents: null }] }), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    expect(container.querySelector(".limit")).toBeNull();
    expect((container.querySelector(".fill") as HTMLElement).style.width).toBe("100%");
  });

  it("with no budget, the bar is the category's share of the month's expenses, the same number as the percentage", async () => {
    serve(summary({ byCategory: [{ categoryId: 1, kind: "expense", cents: 50000, budgetCents: null }, { categoryId: 2, kind: "expense", cents: 10000, budgetCents: null }] }), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    const widths = Array.from(container.querySelectorAll<HTMLElement>(".rail .fill")).map((f) => parseFloat(f.style.width));
    expect(widths[0]).toBeCloseTo(83.33, 1);
    expect(widths[1]).toBeCloseTo(16.67, 1);
  });

  it("a tiny share keeps a visible sliver", async () => {
    serve(summary({ byCategory: [{ categoryId: 1, kind: "expense", cents: 100000, budgetCents: null }, { categoryId: 2, kind: "expense", cents: 10, budgetCents: null }] }), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    expect(container.querySelectorAll<HTMLElement>(".rail .fill")[1].style.minWidth).toBe("2px");
  });

  it("a budgeted category next to one without a budget keeps its spent-vs-budget bar", async () => {
    serve(summary({ byCategory: [{ categoryId: 1, kind: "expense", cents: 104230, budgetCents: 150000 }, { categoryId: 2, kind: "expense", cents: 64000, budgetCents: null }] }), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    const fills = container.querySelectorAll<HTMLElement>(".rail .fill");
    expect(fills[0].style.width).toMatch(/^69\.4/); // 104230 / 150000
    expect(parseFloat(fills[1].style.width)).toBeCloseTo(38.04, 1); // 64000 / 168230
  });
});

describe("Overview: shares and pie", () => {
  const many = (n: number) => {
    const cats = Array.from({ length: n }, (_, i) => ({ id: 100 + i, kind: "expense" as const, name: `Cat ${i + 1}`, icon: "x" }));
    const by = cats.map((c, i) => ({ categoryId: c.id, kind: "expense" as const, cents: (i + 1) * 1000, budgetCents: null }));
    return { ctx: appCtx({ categories: cats as never }), s: summary({ byCategory: by }) };
  };

  it("shows each category's share of the month's expenses next to the amount", async () => {
    serve(summary(), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    const amts = Array.from(container.querySelectorAll(".rail .amt")).map((e) => e.textContent);
    expect(amts[0]).toMatch(/1\.042,30.*62%/); // 104230 / 168230
    expect(amts[1]).toMatch(/640,00.*38%/);
  });

  it("shows <1% for a tiny category", async () => {
    serve(summary({ byCategory: [{ categoryId: 1, kind: "expense", cents: 100000, budgetCents: null }, { categoryId: 2, kind: "expense", cents: 10, budgetCents: null }] }), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    expect(container.querySelectorAll(".rail .amt")[1].textContent).toMatch(/<1%/);
  });

  it("draws the pie below the list, with the points in the aria-label", async () => {
    serve(summary(), [tx()]);
    renderApp(<Overview />);
    const pie = await screen.findByRole("img", { name: /^Gastos por categoria\./ });
    expect(pie.getAttribute("aria-label")).toMatch(/Mercado: R\$\s1\.042,30, 62%; Restaurante: R\$\s640,00, 38%/);
  });

  it("the pie sits in its own card, like the other blocks", async () => {
    serve(summary(), [tx()]);
    renderApp(<Overview />);
    const pie = await screen.findByRole("img", { name: /^Gastos por categoria\./ });
    const card = pie.closest(".chart-card");
    expect(card?.classList.contains("list-group")).toBe(true);
    expect(card?.querySelector(".chart-title")?.textContent).toBe("Gastos por categoria");
  });

  it("has no pie without expenses", async () => {
    serve(summary({ byCategory: [{ categoryId: 3, kind: "income", cents: 850000, budgetCents: null }], expenseCents: 0 }), [tx({ kind: "income", categoryId: 3 })]);
    renderApp(<Overview />);
    await screen.findByRole("heading", { name: "Recentes" });
    expect(screen.queryByText("Gastos por categoria")).toBeNull();
  });

  it("with 8 categories keeps all of them in the pie", async () => {
    const { ctx, s } = many(8);
    serve(s, [tx()]);
    renderApp(<Overview />, ctx);
    const pie = await screen.findByRole("img", { name: /^Gastos por categoria/ });
    expect(pie.getAttribute("aria-label")).not.toMatch(/Outros/);
    expect(pie.getAttribute("aria-label")!.split(";")).toHaveLength(8);
  });

  it("with more than 8 categories groups the smallest as Outros", async () => {
    const { ctx, s } = many(10);
    serve(s, [tx()]);
    renderApp(<Overview />, ctx);
    const pie = await screen.findByRole("img", { name: /^Gastos por categoria/ });
    const label = pie.getAttribute("aria-label")!;
    expect(label.split(";")).toHaveLength(8);
    expect(label).toMatch(/Outros: R\$\s60,00/); // 1000 + 2000 + 3000
    expect(label).toMatch(/Cat 10: R\$\s100,00/);
    expect(label).not.toMatch(/Cat 1:/);
  });
});

describe("Overview: suggested settlement", () => {
  const splitting = (over = {}) => appCtx({ wallet: { ...wallet, splittingEnabled: true }, ...over });
  const withSuggestion = () => summary({ suggestedSettlements: [{ fromUserId: 1, toUserId: 2, amountCents: 19000 }] });

  it("does not appear when the wallet does not split expenses", async () => {
    serve(withSuggestion(), [tx()]);
    renderApp(<Overview />);
    await screen.findByText("Para onde foi");
    expect(screen.queryByText("Acerto entre vocês")).toBeNull();
  });

  it("'Registrar acerto' opens the sheet with the suggestion and today's date; nothing is sent until it is confirmed there", async () => {
    const fetch = serve(withSuggestion(), [tx()]);
    const user = userEvent.setup();
    renderApp(<Overview />, splitting());
    expect(await screen.findByText("Você deve esse valor para Bia.")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Registrar acerto" }));
    const sheet = within(screen.getByRole("dialog"));
    expect(sheet.getByRole("button", { name: /^Quem pagou/ }).textContent).toContain("Ana (você)");
    expect(sheet.getByRole("button", { name: /^Quem recebeu/ }).textContent).toContain("Bia");
    expect((sheet.getByLabelText("Valor do acerto") as HTMLInputElement).value).toBe("190,00");
    expect((sheet.getByLabelText("Data do acerto") as HTMLInputElement).value).toBe(today());
    expect(writes(fetch)).toHaveLength(0);
    await user.click(sheet.getByRole("button", { name: "Registrar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ fromUserId: 1, toUserId: 2, amountCents: 19000, date: today(), note: "" });
  });

  it("when the debt is to you, it says 'a você'", async () => {
    serve(summary({ suggestedSettlements: [{ fromUserId: 2, toUserId: 1, amountCents: 5000 }] }), [tx()]);
    renderApp(<Overview />, splitting());
    expect(await screen.findByText("Bia deve esse valor a você.")).toBeTruthy();
  });

  it("everyone is even when there is no suggestion", async () => {
    serve(summary({ suggestedSettlements: [] }), [tx()]);
    renderApp(<Overview />, splitting());
    expect(await screen.findByText("Vocês estão quites.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Registrar acerto" })).toBeNull();
  });

  it("readers see the settlement, but without the button", async () => {
    serve(withSuggestion(), [tx()]);
    renderApp(<Overview />, splitting({ canEdit: false }));
    expect(await screen.findByText("Você deve esse valor para Bia.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Registrar acerto" })).toBeNull();
  });

  it("shows the API error in the sheet, which stays open", async () => {
    mockFetch((url, init) => {
      if (init.method === "POST") return { status: 400, body: { error: { code: "validation", message: "Acertos exigem a divisão de gastos ativa na carteira" } } };
      return { body: url.includes("/summary") ? withSuggestion() : [tx()] };
    });
    const user = userEvent.setup();
    renderApp(<Overview />, splitting());
    await user.click(await screen.findByRole("button", { name: "Registrar acerto" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Registrar" }));
    expect(await within(screen.getByRole("dialog")).findByText(/Acertos exigem a divisão/)).toBeTruthy();
  });
});

describe("Overview: blocks", () => {
  it("a month with only income shows no empty 'Para onde foi' block", async () => {
    serve(summary({ byCategory: [{ categoryId: 3, kind: "income", cents: 850000, budgetCents: null }], expenseCents: 0 }), [tx({ kind: "income", categoryId: 3 })]);
    renderApp(<Overview />);
    await screen.findByRole("heading", { name: "Recentes" });
    expect(screen.queryByRole("heading", { name: "Para onde foi" })).toBeNull();
  });

  it("'Para onde foi' groups the expense categories in one block", async () => {
    serve(summary(), [tx()]);
    const { container } = renderApp(<Overview />);
    const heading = await screen.findByRole("heading", { name: "Para onde foi" });
    const block = heading.parentElement!.querySelector(".list-group")!;
    expect(block.querySelectorAll(".rail")).toHaveLength(2); // Mercado and Restaurante; the income category is left out
    expect(container.querySelectorAll(".list-group").length).toBeGreaterThanOrEqual(2);
  });

  it("'Recentes' lists at most the 6 latest lines, and tapping one opens it for editing", async () => {
    const many = Array.from({ length: 8 }, (_, i) => tx({ id: i + 1, description: `Compra ${i + 1}` }));
    serve(summary(), many);
    const ctx = appCtx();
    renderApp(<Overview />, ctx);
    expect(await screen.findByRole("heading", { name: "Recentes" })).toBeTruthy();
    expect(screen.getAllByRole("button", { name: /Compra \d/ })).toHaveLength(6);
    await userEvent.click(screen.getByRole("button", { name: /Compra 1/ }));
    expect(ctx.openAdd).toHaveBeenCalledWith(expect.objectContaining({ id: 1 }));
  });

  it("a reader sees the recent lines but cannot open them", async () => {
    serve(summary(), [tx({ description: "Padaria" })]);
    renderApp(<Overview />, appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } }));
    const row = await screen.findByRole("button", { name: /Padaria/ });
    expect((row as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("Overview: what is still to come", () => {
  const next = addMonths(today().slice(0, 7), 1);
  const salary = { recurrenceId: 7, date: `${next}-15`, kind: "income" as const, amountCents: 500000, description: "Salário", categoryId: 3, paymentMethodId: null };
  const internet = { recurrenceId: 8, date: `${next}-28`, kind: "expense" as const, amountCents: 8990, description: "Internet", categoryId: 1, paymentMethodId: 2 };
  const ahead = summary({ upcomingIncomeCents: 500000, upcomingExpenseCents: 8990, upcoming: [salary, internet] });

  it("without anything upcoming, the cards carry no projection", async () => {
    serve(summary(), [tx()]);
    renderApp(<Overview />);
    await screen.findByText("4.376");
    expect(screen.queryByText(/a receber/)).toBeNull();
    expect(screen.queryByText(/a pagar/)).toBeNull();
    expect(screen.queryByText(/Previsto para o fim do mês/)).toBeNull();
    expect(screen.queryByRole("heading", { name: "Próximos" })).toBeNull();
  });

  it("adds what is to receive and to pay under each total, and the forecast under the balance", async () => {
    serve(ahead, [tx()]);
    renderApp(<Overview />);
    await screen.findByText("4.376");
    expect(screen.getByText(/a receber/).textContent).toMatch(/^\+ R\$\s5\.000,00 a receber$/);
    expect(screen.getByText(/a pagar/, { selector: ".ahead" }).textContent).toMatch(/^\+ R\$\s89,90 a pagar$/);
    const forecast = screen.getByText(/Previsto para o fim do mês/);
    expect(forecast.textContent).toMatch(/R\$\s9\.286,70/); // 4.376,60 + 5.000,00 − 89,90
    expect(forecast.querySelector(".negative")).toBeNull();
  });

  it("only the side that has something upcoming gets a line; a negative forecast is red", async () => {
    serve(summary({ balanceCents: -100, upcomingExpenseCents: 8990, upcoming: [internet] }), [tx()]);
    renderApp(<Overview />);
    await screen.findByText(/a pagar/, { selector: ".ahead" });
    expect(screen.queryByText(/a receber/)).toBeNull();
    const value = screen.getByText(/Previsto para o fim do mês/).querySelector(".num")!;
    expect(value.textContent).toMatch(/^-R\$\s90,90$/);
    expect(value.className).toContain("negative");
  });

  it("a future month with only upcoming entries shows the cards and 'Próximos', not the empty state", async () => {
    serve(summary({ month: next, incomeCents: 0, expenseCents: 0, balanceCents: 0, byCategory: [], upcomingIncomeCents: 500000, upcomingExpenseCents: 8990, upcoming: [salary, internet] }), []);
    renderApp(<Overview />, appCtx({ month: next }));
    expect(await screen.findByRole("heading", { name: "Próximos" })).toBeTruthy();
    expect(screen.queryByText(/Nada lançado/)).toBeNull();
    expect(screen.getByText(/a receber/)).toBeTruthy();
    const rows = screen.getAllByText("previsto").map((tag) => tag.closest(".row")!);
    expect(rows.map((r) => r.querySelector(".desc")!.textContent)).toEqual(["Salário", "Internet"]);
    expect(rows[0].querySelector(".val")!.textContent).toMatch(/^\+R\$\s5\.000,00$/);
    expect(rows[1].querySelector(".val")!.textContent).toMatch(/^−R\$\s89,90$/);
    expect(screen.queryByRole("heading", { name: "Recentes" })).toBeNull(); // nothing realized yet
  });

  it("the ruler draws an outlined bar on the days with a projected expense and says the total", async () => {
    serve(summary({ month: next, upcomingIncomeCents: 500000, upcomingExpenseCents: 8990, upcoming: [salary, internet] }), [tx({ date: `${next}-10` })]);
    const { container } = renderApp(<Overview />, appCtx({ month: next }));
    await screen.findByText("4.376");
    const all = ticks(container) as HTMLElement[];
    expect(all.filter((t) => t.classList.contains("projected"))).toEqual([all[27]]); // day 28; income is not drawn
    expect(all[27].style.height).not.toBe("");
    expect(all[27].getAttribute("title")).toMatch(/Dia 28: R\$\s89,90 previsto/);
    const text = container.querySelector(".strip .visually-hidden")!.textContent!;
    expect(text).toMatch(/Previsto: R\$\s89,90 a pagar/);
  });

  it("tapping an upcoming entry opens its recurrence for editing", async () => {
    const rec = { id: 8, kind: "expense", amountCents: 8990, description: "Internet", categoryId: 1, paymentMethodId: 2, paidByUserId: 1, toUserId: null, dayOfMonth: 28, startDate: "2026-01-01", endDate: null, enabled: true, nextDate: null, splits: [] };
    mockFetch((url) => ({ body: url.includes("/summary") ? summary({ upcoming: [internet], upcomingExpenseCents: 8990 }) : url.includes("/recurrences") ? [rec] : [tx()] }));
    renderApp(<Overview />);
    await userEvent.click(await screen.findByRole("button", { name: /Internet/ }));
    expect(await screen.findByRole("dialog", { name: "Editar recorrência" })).toBeTruthy();
  });
});
