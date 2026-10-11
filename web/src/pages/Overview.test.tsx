import { screen, waitFor } from "@testing-library/react";
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
  byEssentiality: {}, previous: { incomeCents: 0, expenseCents: 398000 }, ...over,
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
    await screen.findByText("Por categoria");
    expect(screen.getByText("Mercado")).toBeTruthy();
    expect(screen.getByText("Restaurante")).toBeTruthy();
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
    await screen.findByText("Por categoria");
    const mercado = container.querySelectorAll(".rail")[0];
    expect(mercado.querySelector(".track")!.classList.contains("over")).toBe(false);
    expect((mercado.querySelector(".fill") as HTMLElement).style.width).toMatch(/^69\.4/); // 104230 / 150000
    expect(mercado.querySelector(".warn")).toBeNull();
  });

  it("with no budget, it draws no limit mark", async () => {
    serve(summary({ byCategory: [{ categoryId: 1, kind: "expense", cents: 5000, budgetCents: null }] }), [tx()]);
    const { container } = renderApp(<Overview />);
    await screen.findByText("Por categoria");
    expect(container.querySelector(".limit")).toBeNull();
    expect((container.querySelector(".fill") as HTMLElement).style.width).toBe("100%");
  });
});

describe("Overview: suggested settlement", () => {
  const splitting = (over = {}) => appCtx({ wallet: { ...wallet, splittingEnabled: true }, ...over });
  const withSuggestion = () => summary({ suggestedSettlements: [{ fromUserId: 1, toUserId: 2, amountCents: 19000 }] });

  it("does not appear when the wallet does not split expenses", async () => {
    serve(withSuggestion(), [tx()]);
    renderApp(<Overview />);
    await screen.findByText("Por categoria");
    expect(screen.queryByText("Acerto entre vocês")).toBeNull();
  });

  it("shows who owes whom and registers with today's date", async () => {
    const fetch = serve(withSuggestion(), [tx()]);
    const user = userEvent.setup();
    renderApp(<Overview />, splitting());
    expect(await screen.findByText("Você deve esse valor para Bia.")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Registrar acerto" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ fromUserId: 1, toUserId: 2, amountCents: 19000, date: today() });
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

  it("shows the API error when registering", async () => {
    mockFetch((url, init) => {
      if (init.method === "POST") return { status: 400, body: { error: { code: "validation", message: "Acertos exigem a divisão de gastos ativa na carteira" } } };
      return { body: url.includes("/summary") ? withSuggestion() : [tx()] };
    });
    const user = userEvent.setup();
    renderApp(<Overview />, splitting());
    await user.click(await screen.findByRole("button", { name: "Registrar acerto" }));
    expect(await screen.findByText(/Acertos exigem a divisão/)).toBeTruthy();
  });
});
