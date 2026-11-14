import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Transaction, Upcoming } from "../api/types";
import { addMonths, today } from "../lib/dates";
import { appCtx, mockFetch, renderApp, tx } from "../test/fixtures";
import { Transactions } from "./Transactions";

const items: Transaction[] = [
  tx({ id: 1, description: "Padaria", amountCents: 1800 }),
  tx({ id: 2, description: "Feira da semana", amountCents: 28150 }),
  tx({ id: 3, kind: "income", categoryId: 3, description: "Salário", amountCents: 850000 }),
];

const noneAhead = { upcomingIncomeCents: 0, upcomingExpenseCents: 0, upcoming: [] };
const serve = (list: Transaction[] = items) => mockFetch((url) => ({ body: url.includes("/recurrences") ? [] : url.includes("/summary") ? noneAhead : list }));
const rows = () => screen.queryAllByRole("button", { name: /Padaria|Feira|Salário/ });

describe("Transactions", () => {
  it("totals the month's income and expenses and lists everything by default", async () => {
    serve();
    renderApp(<Transactions />);
    expect(await screen.findByText("Padaria")).toBeTruthy();
    expect(screen.getByText(/Entrou/).textContent).toMatch(/8\.500,00/);
    expect(screen.getByText(/Saiu/).textContent).toMatch(/299,50/); // 18.00 + 281.50
    expect(rows()).toHaveLength(3);
    expect(screen.getByRole("button", { name: "Tudo" }).getAttribute("aria-pressed")).toBe("true");
  });

  it("searches the description case-insensitively", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<Transactions />);
    await screen.findByText("Padaria");
    await user.type(screen.getByRole("searchbox", { name: "Buscar" }), "FEIRA");
    expect(rows().map((r) => r.textContent)).toEqual([expect.stringContaining("Feira da semana")]);
  });

  it("also finds by category and payment method name, ignoring accents and case", async () => {
    serve([...items, tx({ id: 4, description: "", categoryId: 2, paymentMethodId: 2, amountCents: 5000 })]);
    const user = userEvent.setup();
    renderApp(<Transactions />);
    await screen.findByText("Padaria");
    const box = screen.getByRole("searchbox", { name: "Buscar" });
    const found = () => screen.getAllByRole("button", { name: /−R\$|\+R\$/ }).map((r) => r.querySelector(".desc")?.textContent);
    await user.type(box, "RESTAURANTE");
    expect(found()).toEqual(["Restaurante"]); // no description: the row shows the category name
    await user.clear(box);
    await user.type(box, "credito");
    expect(found()).toEqual(["Restaurante"]);
    await user.clear(box);
    await user.type(box, "salario");
    expect(found()).toEqual(["Salário"]);
  });

  it("filters by expenses and by income", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<Transactions />);
    await screen.findByText("Padaria");

    await user.click(screen.getByRole("button", { name: "Gastos" }));
    expect(rows()).toHaveLength(2);
    expect(screen.queryByText("Salário")).toBeNull();
    expect(screen.getByRole("button", { name: "Gastos" }).getAttribute("aria-pressed")).toBe("true");

    await user.click(screen.getByRole("button", { name: "Entradas" }));
    expect(rows().map((r) => r.textContent)).toEqual([expect.stringContaining("Salário")]);
  });

  it("combines filter and search; the totals stay those of the whole month", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<Transactions />);
    await screen.findByText("Padaria");
    await user.click(screen.getByRole("button", { name: "Gastos" }));
    await user.type(screen.getByRole("searchbox", { name: "Buscar" }), "sal"); // only "Salário", which is income
    expect(screen.getByText("Nenhum lançamento corresponde ao filtro.")).toBeTruthy();
    expect(screen.getByText(/Entrou/).textContent).toMatch(/8\.500,00/);
  });

  it("a month with no transactions invites creating the first one", async () => {
    serve([]);
    const ctx = appCtx();
    const user = userEvent.setup();
    renderApp(<Transactions />, ctx);
    expect(await screen.findByText("Nenhum lançamento neste mês.")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Novo lançamento" }));
    expect(ctx.openAdd).toHaveBeenCalledWith();
  });

  it("readers do not get the invitation to create", async () => {
    serve([]);
    renderApp(<Transactions />, appCtx({ canEdit: false }));
    expect(await screen.findByText("Nenhum lançamento neste mês.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Novo lançamento" })).toBeNull();
  });

  it("asks the API for the right month", async () => {
    const fetch = serve([]);
    renderApp(<Transactions />, appCtx({ month: "2026-02" }));
    await screen.findByText("Nenhum lançamento neste mês.");
    expect(fetch.mock.calls.map((c) => c[0])).toContain("/api/wallets/1/transactions?from=2026-02-01&to=2026-02-28");
  });

  it("groups by day, with today identified", async () => {
    serve([tx({ id: 1, date: today() })]);
    renderApp(<Transactions />);
    expect(await screen.findByText(/^Hoje,/)).toBeTruthy();
  });

  it("shows the error with a retry option", async () => {
    const fetch = mockFetch({ status: 500, body: { error: { code: "internal", message: "Erro interno" } } });
    const user = userEvent.setup();
    renderApp(<Transactions />);
    expect(await screen.findByText("Não foi possível carregar os lançamentos.")).toBeTruthy();
    const before = fetch.mock.calls.length;
    await user.click(screen.getByRole("button", { name: "Tentar de novo" }));
    await waitFor(() => expect(fetch.mock.calls.length).toBeGreaterThan(before));
  });

  it("shows loading while the API has not answered", () => {
    mockFetch(() => new Promise(() => {}) as never);
    const { container } = renderApp(<Transactions />);
    expect(container.querySelectorAll(".skeleton").length).toBeGreaterThan(0);
    expect(screen.queryByRole("searchbox")).toBeNull();
  });
});

describe("Transactions: recurrences section", () => {
  it("shows 'Repetem todo mês' under the list, even in a month with no transactions", async () => {
    serve([]);
    renderApp(<Transactions />);
    expect(await screen.findByText("Nenhum lançamento neste mês.")).toBeTruthy();
    expect(await screen.findByRole("heading", { name: "Repetem todo mês" })).toBeTruthy();
  });
});

describe("Transactions: new look", () => {
  it("the filter is a segmented control with Tudo, Gastos and Entradas", async () => {
    serve();
    const { container } = renderApp(<Transactions />);
    await screen.findByText("Padaria");
    const group = container.querySelector('.segmented[aria-label="Filtros"]')!;
    expect([...group.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["Tudo", "Gastos", "Entradas"]);
    expect([...group.querySelectorAll("button")].filter((b) => b.getAttribute("aria-pressed") === "true").map((b) => b.textContent)).toEqual(["Tudo"]);
  });

  it("the totals use the same marked-up line as the overview", async () => {
    serve();
    const { container } = renderApp(<Transactions />);
    await screen.findByText("Padaria");
    expect(container.querySelector(".flows .in")?.textContent).toMatch(/Entrou.*8.500,00/);
    expect(container.querySelector(".flows .out")?.textContent).toMatch(/Saiu/); // the expense total is red, like every expense
    expect(container.querySelectorAll(".flows span")).toHaveLength(2);
  });
});

describe("Transactions: Próximos", () => {
  const next = addMonths(today().slice(0, 7), 1);
  const upcoming: Upcoming[] = [
    { recurrenceId: 7, date: `${next}-15`, kind: "income", amountCents: 500000, description: "Salário", categoryId: 3, paymentMethodId: null },
    { recurrenceId: 8, date: `${next}-28`, kind: "expense", amountCents: 8990, description: "Internet", categoryId: 2, paymentMethodId: 2 },
  ];
  const serveAhead = (list: Transaction[] = items) =>
    mockFetch((url) => ({ body: url.includes("/recurrences") ? [] : url.includes("/summary") ? { upcomingIncomeCents: 500000, upcomingExpenseCents: 8990, upcoming } : list }));
  const ahead = () => {
    const h = screen.queryByRole("heading", { name: "Próximos" });
    return h ? Array.from(h.parentElement!.querySelectorAll(".row .desc")).map((d) => d.textContent) : [];
  };

  it("lists the month's upcoming entries above the realized ones, marked 'previsto', and the totals show what is to come", async () => {
    serveAhead();
    renderApp(<Transactions />, appCtx({ month: next }));
    expect(await screen.findByRole("heading", { name: "Próximos" })).toBeTruthy();
    expect(ahead()).toEqual(["Salário", "Internet"]);
    expect(screen.getAllByText("previsto")).toHaveLength(2);
    const heading = screen.getByRole("heading", { name: "Próximos" });
    expect(heading.compareDocumentPosition(screen.getByText("Padaria")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // the realized ones get their own title, so they do not read as part of Próximos; their days sit under it
    const done = screen.getByRole("heading", { name: "Lançados", level: 2 });
    expect(heading.compareDocumentPosition(done) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(done.parentElement!.querySelector("h3")).toBeTruthy();
    expect(screen.getByText(/a receber/).textContent).toMatch(/^\+ R\$\s5\.000,00 a receber$/);
    expect(screen.getByText(/a pagar/).textContent).toMatch(/^\+ R\$\s89,90 a pagar$/);
  });

  it("without anything to come there is no Lançados title: the list stands alone as before", async () => {
    mockFetch((url) => ({ body: url.includes("/recurrences") ? [] : url.includes("/summary") ? { upcomingIncomeCents: 0, upcomingExpenseCents: 0, upcoming: [] } : items }));
    renderApp(<Transactions />);
    await screen.findByText("Padaria");
    expect(screen.queryByRole("heading", { name: "Lançados" })).toBeNull();
  });

  it("follows the filters and the search", async () => {
    serveAhead();
    const user = userEvent.setup();
    renderApp(<Transactions />, appCtx({ month: next }));
    await screen.findByRole("heading", { name: "Próximos" });
    await user.click(screen.getByRole("button", { name: "Gastos" }));
    expect(ahead()).toEqual(["Internet"]);
    await user.click(screen.getByRole("button", { name: "Entradas" }));
    expect(ahead()).toEqual(["Salário"]);
    await user.click(screen.getByRole("button", { name: "Tudo" }));
    await user.type(screen.getByRole("searchbox", { name: "Buscar" }), "credito"); // the payment method
    expect(ahead()).toEqual(["Internet"]);
    await user.clear(screen.getByRole("searchbox", { name: "Buscar" }));
    await user.type(screen.getByRole("searchbox", { name: "Buscar" }), "padaria");
    expect(screen.queryByRole("heading", { name: "Próximos" })).toBeNull();
  });
});
