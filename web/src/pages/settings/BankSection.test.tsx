import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { BankAccount } from "../../api/types";
import { appCtx, mockFetch, renderApp, wallet } from "../../test/fixtures";
import { BankSection } from "./BankSection";

const account: BankAccount = { id: 3, name: "Nubank", paymentMethodId: null, paidByUserId: 1, enabled: true, pendingCount: 2 };
const rule = { id: 8, pattern: "uber", categoryId: 1, renameTo: "Uber", priority: 0 };
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");

function serve(rules: unknown[] = []) {
  return mockFetch((url, init) => {
    if (init.method === "DELETE") return { status: 204 };
    if (init.method && init.method !== "GET") return { body: {} };
    return { body: url.endsWith("/bank-accounts") ? [account] : url.endsWith("/rules") ? rules : [] };
  });
}
const reader = () => appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } });

describe("Bank accounts screen", () => {
  it("lists the accounts with what waits for review", async () => {
    serve();
    renderApp(<BankSection />);
    const row = await screen.findByRole("button", { name: /Nubank/ });
    expect(row.textContent).toContain("2 para revisar");
  });

  it("an account with nothing pending says it is up to date, and a disabled one says so", async () => {
    mockFetch({ body: [{ ...account, pendingCount: 0 }, { ...account, id: 4, name: "Itaú", enabled: false }] });
    renderApp(<BankSection />);
    expect((await screen.findByRole("button", { name: /Nubank/ })).textContent).toContain("em dia");
    expect(screen.getByRole("button", { name: /Itaú/ }).textContent).toContain("desativada");
  });

  it("tapping an account opens its sheet", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.click(await screen.findByRole("button", { name: /Nubank/ }));
    expect(screen.getByLabelText("Arquivo do extrato")).toBeTruthy();
  });

  it("'Nova conta bancária' opens an empty sheet", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.click(await screen.findByRole("button", { name: "Nova conta bancária" }));
    expect(screen.getByLabelText("Nome da conta bancária")).toBeTruthy();
  });

  it("lists the category rules and deletes one", async () => {
    const fetch = serve([rule]);
    const user = userEvent.setup();
    renderApp(<BankSection />);
    expect(await screen.findByText(/renomeia para Uber/)).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Excluir regra uber" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/rules/8");
  });

  it("a reader sees accounts and rules but cannot open, create or delete", async () => {
    serve([rule]);
    renderApp(<BankSection />, reader());
    expect(await screen.findByText("Nubank")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Nubank/ })).toBeNull();
    expect(screen.queryByRole("button", { name: "Nova conta bancária" })).toBeNull();
    expect(await screen.findByText(/renomeia para Uber/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Excluir regra uber" })).toBeNull();
  });

  it("a load failure is said, not silent", async () => {
    mockFetch({ status: 500, body: { error: { code: "x", message: "falhou" } } });
    renderApp(<BankSection />);
    expect(await screen.findByText("Não foi possível carregar as contas.")).toBeTruthy();
  });
});
