import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { InboxItem } from "../api/types";
import { appCtx, mockFetch, renderApp, wallet } from "../test/fixtures";
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

  it("lists the lines in one block, with the suggested name and what the statement said", async () => {
    serve([line({ id: 1, suggestedCategoryId: 1, suggestedDescription: "Mercado" }), line({ id: 2, description: "UBER *TRIP", amountCents: -2500 })]);
    const { container } = renderApp(<Inbox />);
    expect(await screen.findByText("Mercado")).toBeTruthy();
    expect(screen.getByText(/No extrato: COMPRA MERCADO DO BAIRRO/)).toBeTruthy();
    expect(screen.getByText("UBER *TRIP")).toBeTruthy();
    expect(container.querySelectorAll(".list-group > .list-row")).toHaveLength(2);
  });

  it("tapping a line opens the decision sheet for that line", async () => {
    serve([line()]);
    const user = userEvent.setup();
    renderApp(<Inbox />);
    await user.click(await screen.findByRole("button", { name: /COMPRA MERCADO DO BAIRRO/ }));
    expect(screen.getByRole("button", { name: "Aceitar" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Ignorar" })).toBeTruthy();
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

  it("with 250 lines it lists them all and counts only the safe ones", async () => {
    const many = Array.from({ length: 250 }, (_, i) => line({ id: i + 1, description: `LINHA ${i + 1}`, suggestedCategoryId: i % 5 === 0 ? 1 : null }));
    serve(many);
    const { container } = renderApp(<Inbox />);
    await screen.findByRole("button", { name: "Aceitar 50 sugeridas" });
    expect(container.querySelectorAll(".list-group > .list-row")).toHaveLength(250);
  });

  it("shows the error when the bulk accept fails", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 500, body: { error: { code: "internal", message: "Erro interno" } } } : { body: [line({ suggestedCategoryId: 1 })] }));
    const user = userEvent.setup();
    renderApp(<Inbox />);
    await user.click(await screen.findByRole("button", { name: "Aceitar 1 sugerida" }));
    expect(await screen.findByText("Erro interno")).toBeTruthy();
  });

  it("a reader can look but not decide", async () => {
    serve([line({ suggestedCategoryId: 1 })]);
    renderApp(<Inbox />, appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } }));
    expect(await screen.findByText("COMPRA MERCADO DO BAIRRO")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /COMPRA MERCADO DO BAIRRO/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Aceitar/ })).toBeNull();
  });
});
