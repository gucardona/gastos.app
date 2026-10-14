import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Category } from "../api/types";
import { bodyOf, categories, mockFetch, renderApp } from "../test/fixtures";
import { BudgetSheet } from "./BudgetSheet";

const mercado = categories[0];
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const serve = () => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: [] }));

function open(category: Category | null, current?: number) {
  const onClose = vi.fn();
  renderApp(<BudgetSheet category={category} current={current} onClose={onClose} />);
  return { onClose, user: userEvent.setup() };
}
const field = () => screen.getByLabelText("Limite mensal de Mercado") as HTMLInputElement;

describe("BudgetSheet", () => {
  it("is closed without a category", () => {
    open(null);
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("starts from the current limit, and saves a new one in cents", async () => {
    const fetch = serve();
    const { user, onClose } = open(mercado, 150000);
    expect(field().value).toBe("1500,00");
    await user.clear(field());
    await user.type(field(), "1.800,50");
    await user.click(screen.getByRole("button", { name: "Definir" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/budgets/1");
    expect(writes(fetch)[0][1]!.method).toBe("PUT");
    expect(bodyAt(fetch)).toEqual({ limitCents: 180050 });
  });

  it.each(["abc", "0", ""])("%j is refused before anything is sent", async (typed) => {
    const fetch = serve();
    const { user } = open(mercado);
    if (typed) await user.type(field(), typed);
    await user.click(screen.getByRole("button", { name: "Definir" }));
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("removes the limit, and offers it only when there is one", async () => {
    const fetch = serve();
    const { user, onClose } = open(mercado, 150000);
    await user.click(screen.getByRole("button", { name: "Remover limite" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/budgets/1");
  });

  it("no remove button without a limit", () => {
    open(mercado);
    expect(screen.queryByRole("button", { name: "Remover limite" })).toBeNull();
  });

  it("shows the error of the API and stays open", async () => {
    mockFetch((_u, init) => (init.method === "PUT" ? { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { limitCents: "valor muito alto" } } } } : { body: [] }));
    const { user, onClose } = open(mercado);
    await user.type(field(), "10");
    await user.click(screen.getByRole("button", { name: "Definir" }));
    expect(await screen.findByText("valor muito alto")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });
});
