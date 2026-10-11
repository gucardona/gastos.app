import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Transaction } from "../api/types";
import { today } from "../lib/dates";
import { appCtx, mockFetch, renderApp, tx } from "../test/fixtures";
import { Transactions } from "./Transactions";

const items: Transaction[] = [
  tx({ id: 1, description: "Padaria", amountCents: 1800 }),
  tx({ id: 2, description: "Feira da semana", amountCents: 28150 }),
  tx({ id: 3, kind: "income", categoryId: 3, description: "Salário", amountCents: 850000 }),
];

const serve = (list: Transaction[] = items) => mockFetch({ body: list });
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
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/transactions?from=2026-02-01&to=2026-02-28");
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
