import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../../test/fixtures";
import { WalletSection } from "./WalletSection";

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const ok = () => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: [] }));
const editor = () => appCtx({ wallet: { ...wallet, role: "editor" } });

describe("Wallet screen", () => {
  it("renames the wallet, only once the name changed", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    expect(screen.queryByRole("button", { name: "Salvar nome" })).toBeNull();
    await user.clear(screen.getByLabelText("Nome da carteira"));
    await user.type(screen.getByLabelText("Nome da carteira"), "Casa nova");
    await user.click(screen.getByRole("button", { name: "Salvar nome" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1");
    expect(bodyAt(fetch)).toEqual({ name: "Casa nova" });
  });

  it("turns expense splitting on", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("switch", { name: "Dividir gastos entre os membros" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][1]!.method).toBe("PATCH");
    expect(bodyAt(fetch)).toEqual({ splittingEnabled: true });
  });

  it("creates a new wallet from a sheet and starts using it", async () => {
    const created = { ...wallet, id: 9, name: "Viagem" };
    const ctx = appCtx();
    const fetch = mockFetch((_u, init) => (init.method === "POST" ? { status: 201, body: created } : { body: [] }));
    const user = userEvent.setup();
    renderApp(<WalletSection />, ctx);
    await user.click(screen.getByRole("button", { name: "Nova carteira" }));
    await user.type(screen.getByLabelText("Nome da nova carteira"), "Viagem");
    await user.click(screen.getByRole("button", { name: "Criar carteira" }));
    await waitFor(() => expect(ctx.setWalletId).toHaveBeenCalledWith(9));
    expect(writes(fetch)[0][0]).toBe("/api/wallets");
    expect(bodyAt(fetch)).toEqual({ name: "Viagem", splittingEnabled: false });
  });

  it("the new wallet can start with splitting on", async () => {
    const fetch = mockFetch((_u, init) => (init.method === "POST" ? { status: 201, body: { ...wallet, id: 9 } } : { body: [] }));
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("button", { name: "Nova carteira" }));
    await user.type(screen.getByLabelText("Nome da nova carteira"), "Viagem");
    await user.click(screen.getByRole("switch", { name: "Dividir gastos na nova carteira" }));
    await user.click(screen.getByRole("button", { name: "Criar carteira" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch).splittingEnabled).toBe(true);
  });

  it("shows the error of the API when creating, and keeps the sheet open", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { name: "obrigatório" } } } } : { body: [] }));
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("button", { name: "Nova carteira" }));
    await user.type(screen.getByLabelText("Nome da nova carteira"), "x");
    await user.click(screen.getByRole("button", { name: "Criar carteira" }));
    expect(await screen.findByText("obrigatório")).toBeTruthy();
  });

  it("deletes the wallet only after confirming", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("button", { name: "Excluir esta carteira" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1");
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
  });

  it("warns when it is the only wallet", () => {
    renderApp(<WalletSection />);
    expect(screen.getByText(/É a sua única carteira/)).toBeTruthy();
  });

  it("someone who is not the owner cannot rename, switch splitting or delete", () => {
    renderApp(<WalletSection />, editor());
    expect((screen.getByLabelText("Nome da carteira") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByRole("switch", { name: "Dividir gastos entre os membros" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByRole("button", { name: "Excluir esta carteira" })).toBeNull();
  });
});

describe("Wallet screen: a double click is not a confirmation", () => {
  it("clicking Excluir esta carteira twice fast does not delete anything", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.dblClick(screen.getByRole("button", { name: "Excluir esta carteira" }));
    expect(writes(fetch)).toHaveLength(0);
    expect(screen.getByRole("button", { name: "Confirmar exclusão" })).toBeTruthy();
  });
});
