import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { InboxItem } from "../api/types";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../test/fixtures";
import { Inbox } from "./Inbox";

const line = (over: Partial<InboxItem> = {}): InboxItem => ({
  id: 5, bankAccountId: 1, accountName: "Nubank", date: "2026-10-01", amountCents: -4890, description: "COMPRA MERCADO DO BAIRRO",
  status: "pending", suggestedCategoryId: null, suggestedDescription: null, suggestedMatch: null, ...over,
});

const serve = (items: InboxItem[]) => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: items }));
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");

describe("Inbox", () => {
  it("points to the settings when there is nothing to review", async () => {
    serve([]);
    renderApp(<Inbox />);
    expect(await screen.findByText(/Nada para revisar/)).toBeTruthy();
    expect(screen.getByRole("link", { name: /Contas bancárias/ }).getAttribute("href")).toBe("/ajustes?s=bancos");
  });

  it("accepts a line with the chosen category", async () => {
    const fetch = serve([line()]);
    const user = userEvent.setup();
    renderApp(<Inbox />);
    await user.selectOptions(await screen.findByLabelText("Categoria de COMPRA MERCADO DO BAIRRO"), "1");
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/inbox/5/accept");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ categoryId: 1 });
  });

  it("cannot accept before a category is chosen", async () => {
    serve([line()]);
    renderApp(<Inbox />);
    expect((await screen.findByRole("button", { name: "Aceitar" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("only offers categories of the line's kind", async () => {
    serve([line({ amountCents: 850000, description: "SALARIO" })]);
    renderApp(<Inbox />);
    const select = await screen.findByLabelText("Categoria de SALARIO");
    expect([...select.querySelectorAll("option")].map((o) => o.textContent)).toEqual(["Escolha", "💼 Salário"]);
  });

  it("starts from the suggested category and name", async () => {
    serve([line({ suggestedCategoryId: 1, suggestedDescription: "Mercado" })]);
    renderApp(<Inbox />);
    expect(await screen.findByText("Mercado")).toBeTruthy();
    expect(screen.getByText(/No extrato: COMPRA MERCADO DO BAIRRO/)).toBeTruthy();
    expect((screen.getByLabelText("Categoria de Mercado") as HTMLSelectElement).value).toBe("1");
  });

  it("creates a rule when asked, after accepting", async () => {
    const fetch = serve([line()]);
    const user = userEvent.setup();
    renderApp(<Inbox />);
    await user.selectOptions(await screen.findByLabelText("Categoria de COMPRA MERCADO DO BAIRRO"), "1");
    await user.click(screen.getByLabelText(/Usar esta categoria/));
    expect((screen.getByLabelText("Texto da regra") as HTMLInputElement).value).toBe("compra mercado");
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(2));
    expect(writes(fetch)[1][0]).toBe("/api/wallets/1/rules");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[1]))).toEqual({ pattern: "compra mercado", categoryId: 1 });
  });

  it("links to the look-alike transaction instead of creating one", async () => {
    const fetch = serve([line({ suggestedMatch: { id: 77, description: "Mercado (digitado)", date: "2026-10-02" } })]);
    const user = userEvent.setup();
    renderApp(<Inbox />);
    expect(await screen.findByText(/Parece o lançamento que você já fez: Mercado \(digitado\)/)).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "É este lançamento" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/inbox/5/match");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ transactionId: 77 });
  });

  it("ignores a line", async () => {
    const fetch = serve([line()]);
    const user = userEvent.setup();
    renderApp(<Inbox />);
    await user.click(await screen.findByRole("button", { name: "Ignorar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/inbox/5/ignore");
  });

  it("accepts all the safe suggestions at once, not counting lines that need a decision", async () => {
    const fetch = serve([
      line({ id: 1, suggestedCategoryId: 1 }),
      line({ id: 2, suggestedCategoryId: 1, suggestedMatch: { id: 9, description: "x", date: "2026-10-01" } }),
      line({ id: 3 }),
    ]);
    const user = userEvent.setup();
    renderApp(<Inbox />);
    await user.click(await screen.findByRole("button", { name: "Aceitar 1 sugerida" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/inbox/accept-suggested");
  });

  it("shows the server's error", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 409, body: { error: { code: "conflict", message: "Esta linha já foi tratada" } } } : { body: [line()] }));
    const user = userEvent.setup();
    renderApp(<Inbox />);
    await user.click(await screen.findByRole("button", { name: "Ignorar" }));
    expect(await screen.findByText("Esta linha já foi tratada")).toBeTruthy();
  });

  it("a reader can look but not decide", async () => {
    serve([line()]);
    renderApp(<Inbox />, appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } }));
    expect(await screen.findByText("COMPRA MERCADO DO BAIRRO")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Aceitar" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Ignorar" })).toBeNull();
  });
});
