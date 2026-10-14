import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, bodyOf, categories, mockFetch, renderApp, wallet } from "../../test/fixtures";
import { BudgetsSection } from "./BudgetsSection";

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const withBudget = () => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: [{ categoryId: 1, limitCents: 150000 }] }));

describe("Budgets screen", () => {
  it("lists the active expense categories with their limit, or a note that there is none", async () => {
    withBudget();
    renderApp(<BudgetsSection />);
    expect((await screen.findByRole("button", { name: /Mercado/ })).textContent).toMatch(/1\.500,00/);
    expect(screen.getByRole("button", { name: /Restaurante/ }).textContent).toContain("sem limite");
    expect(screen.queryByText("Salário")).toBeNull();
  });

  it("an archived expense category is not listed", async () => {
    withBudget();
    renderApp(<BudgetsSection />, appCtx({ categories: [categories[0], { ...categories[1], archived: true }, categories[2]] }));
    await screen.findByRole("button", { name: /Mercado/ });
    expect(screen.queryByText("Restaurante")).toBeNull();
  });

  it("tapping a category opens its limit and saves a new one in cents", async () => {
    const fetch = withBudget();
    const user = userEvent.setup();
    renderApp(<BudgetsSection />);
    await user.click(await screen.findByRole("button", { name: /Mercado/ }));
    const input = screen.getByLabelText("Limite mensal de Mercado") as HTMLInputElement;
    expect(input.value).toBe("1500,00");
    await user.clear(input);
    await user.type(input, "1.800,50");
    await user.click(screen.getByRole("button", { name: "Definir" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ limitCents: 180050 });
  });

  it("a reader sees the limits but cannot open them", async () => {
    withBudget();
    renderApp(<BudgetsSection />, appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } }));
    expect(await screen.findByText("Mercado")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Mercado/ })).toBeNull();
  });

  it("a load failure offers to try again", async () => {
    mockFetch({ status: 500, body: { error: { code: "x", message: "falhou" } } });
    renderApp(<BudgetsSection />);
    expect(await screen.findByText("Não foi possível carregar os orçamentos.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeTruthy();
  });
});
