import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../../test/fixtures";
import { BudgetsSection } from "./BudgetsSection";
import { MembersSection } from "./MembersSection";
import { WalletSection } from "./WalletSection";

const writes = (fetch: ReturnType<typeof mockFetch>) => fetch.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const lastWrite = (fetch: ReturnType<typeof mockFetch>) => {
  const w = writes(fetch);
  return { url: w[w.length - 1][0] as string, method: w[w.length - 1][1]!.method, body: w[w.length - 1][1]!.body ? bodyOf(fetch, fetch.mock.calls.indexOf(w[w.length - 1])) : undefined };
};
const ok = (extra: Record<string, unknown> = {}) => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: extra } : { body: [] }));

describe("Settings: members", () => {
  it("invites by email with the chosen permission", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.type(screen.getByLabelText("E-mail do convidado"), "caio@example.com");
    await user.selectOptions(screen.getByLabelText("Permissão do convidado"), "reader");
    await user.click(screen.getByRole("button", { name: "Convidar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/members", method: "POST", body: { email: "caio@example.com", role: "reader" } });
  });

  it("shows the error when the email has no account", async () => {
    mockFetch({ status: 404, body: { error: { code: "not_found", message: "Usuário não encontrado" } } });
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.type(screen.getByLabelText("E-mail do convidado"), "x@example.com");
    await user.click(screen.getByRole("button", { name: "Convidar" }));
    expect(await screen.findByText("Usuário não encontrado")).toBeTruthy();
  });

  it("the owner changes another member's permission, but not their own", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    expect(screen.queryByLabelText("Permissão de Ana Souza")).toBeNull();
    await user.selectOptions(screen.getByLabelText("Permissão de Bia Lima"), "reader");
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/members/2", method: "PATCH", body: { role: "reader" } });
  });

  it("a non-owner cannot invite or remove anyone, but can leave", () => {
    const bia = { ...wallet, role: "editor" as const };
    renderApp(<MembersSection />, appCtx({ user: { id: 2, name: "Bia Lima", email: "bia@example.com" }, wallet: bia }));
    expect(screen.queryByRole("button", { name: "Convidar" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Remover" })).toBeNull();
    expect(screen.getByRole("button", { name: "Sair da carteira" })).toBeTruthy();
  });
});

describe("Settings: budgets", () => {
  const withBudget = () => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: [{ categoryId: 1, limitCents: 150000 }] }));

  it("shows the current limit and saves a new one in cents", async () => {
    const fetch = withBudget();
    const user = userEvent.setup();
    renderApp(<BudgetsSection />);
    const input = (await screen.findByLabelText("Limite mensal de Mercado")) as HTMLInputElement;
    expect(input.value).toBe("1500,00");
    await user.clear(input);
    await user.type(input, "1.800,50");
    await user.click(input.closest("form")!.querySelector("button")!);
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/budgets/1", method: "PUT", body: { limitCents: 180050 } });
  });

  it("rejects an invalid value without calling the API", async () => {
    const fetch = withBudget();
    const user = userEvent.setup();
    renderApp(<BudgetsSection />);
    const input = await screen.findByLabelText("Limite mensal de Restaurante");
    await user.type(input, "abc");
    await user.click(input.closest("form")!.querySelector("button")!);
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("removes the limit", async () => {
    const fetch = withBudget();
    const user = userEvent.setup();
    renderApp(<BudgetsSection />);
    await screen.findByLabelText("Limite mensal de Mercado");
    await user.click(screen.getByRole("button", { name: "Remover" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/budgets/1", method: "DELETE" });
  });

  it("lists only active expense categories", async () => {
    withBudget();
    renderApp(<BudgetsSection />);
    await screen.findByLabelText("Limite mensal de Mercado");
    expect(screen.queryByLabelText("Limite mensal de Salário")).toBeNull();
  });
});

describe("Settings: wallet", () => {
  it("turns expense splitting on", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("switch", { name: "Dividir gastos entre os membros" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1", method: "PATCH", body: { splittingEnabled: true } });
  });

  it("creates a new wallet and starts using it", async () => {
    const created = { ...wallet, id: 9, name: "Viagem" };
    const ctx = appCtx();
    const fetch = mockFetch((_u, init) => (init.method === "POST" ? { status: 201, body: created } : { body: [] }));
    const user = userEvent.setup();
    renderApp(<WalletSection />, ctx);
    await user.type(screen.getByLabelText("Nome da nova carteira"), "Viagem");
    await user.click(screen.getByRole("button", { name: "Criar carteira" }));
    await waitFor(() => expect(ctx.setWalletId).toHaveBeenCalledWith(9));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets", method: "POST", body: { name: "Viagem", splittingEnabled: false } });
  });

  it("deletes the wallet only after confirming", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("button", { name: "Excluir" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1", method: "DELETE" });
  });

  it("a non-owner cannot edit the wallet", () => {
    renderApp(<WalletSection />, appCtx({ wallet: { ...wallet, role: "editor" } }));
    expect(screen.queryByRole("button", { name: "Excluir" })).toBeNull();
    expect((screen.getByLabelText("Nome da carteira") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByRole("switch", { name: "Dividir gastos entre os membros" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
