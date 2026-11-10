import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Member } from "../api/types";
import { appCtx, bodyOf, members, mockFetch, renderApp, wallet } from "../test/fixtures";
import { MemberSheet } from "./MemberSheet";

const [ana, bia] = members; // Ana owns the wallet, Bia is an editor
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const serve = () => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: [] }));

function open(member: Member | null, ctx = appCtx()) {
  const onClose = vi.fn();
  renderApp(<MemberSheet member={member} onClose={onClose} />, ctx);
  return { onClose, user: userEvent.setup() };
}
const asBia = () => appCtx({ user: { id: 2, name: "Bia Lima", email: "bia@example.com" }, wallet: { ...wallet, role: "editor" } });

describe("MemberSheet: the owner looking at someone else", () => {
  it("is closed without a member", () => {
    open(null);
    expect(screen.queryByRole("group", { name: "Permissão" })).toBeNull();
  });

  it("changes the permission with a segmented control", async () => {
    const fetch = serve();
    const { user } = open(bia);
    expect(screen.getByRole("button", { name: "Pode editar" }).getAttribute("aria-pressed")).toBe("true");
    await user.click(screen.getByRole("button", { name: "Só leitura" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/members/2");
    expect(writes(fetch)[0][1]!.method).toBe("PATCH");
    expect(bodyAt(fetch)).toEqual({ role: "reader" });
    expect(screen.getByRole("button", { name: "Só leitura" }).getAttribute("aria-pressed")).toBe("true");
  });

  it("the wallet owner has no permission to change", () => {
    open(ana);
    expect(screen.queryByRole("group", { name: "Permissão" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Remover" })).toBeNull();
  });

  it("the default split switch exists only when the wallet splits expenses", async () => {
    const fetch = serve();
    const { user } = open(bia, appCtx({ wallet: { ...wallet, splittingEnabled: true } }));
    await user.click(screen.getByRole("switch", { name: "Dividir gastos com Bia Lima por padrão" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toEqual({ splitsByDefault: false });
  });

  it("no default split switch without splitting", () => {
    open(bia);
    expect(screen.queryByRole("switch")).toBeNull();
  });

  it("removes only after confirming, then closes", async () => {
    const fetch = serve();
    const { user, onClose } = open(bia);
    await user.click(screen.getByRole("button", { name: "Remover" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar remoção" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/members/2");
  });

  it("shows the error of the API and stays open", async () => {
    mockFetch((_u, init) => (init.method === "PATCH" ? { status: 403, body: { error: { code: "forbidden", message: "Sem permissão para esta ação" } } } : { body: [] }));
    const { user, onClose } = open(bia);
    await user.click(screen.getByRole("button", { name: "Só leitura" }));
    expect(await screen.findByText("Sem permissão para esta ação")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("MemberSheet: someone who is not the owner", () => {
  it("can only leave the wallet, after confirming", async () => {
    const fetch = serve();
    const { user, onClose } = open(bia, asBia());
    expect(screen.queryByRole("group", { name: "Permissão" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Sair da carteira" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar saída" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/members/2");
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
  });
});

describe("MemberSheet: a double click is not a confirmation", () => {
  it("clicking Remover twice fast does not remove anyone", async () => {
    const fetch = serve();
    const { user } = open(bia);
    await user.dblClick(screen.getByRole("button", { name: "Remover" }));
    expect(writes(fetch)).toHaveLength(0);
  });
});

describe("MemberSheet: default share", () => {
  const ctx = () => appCtx({ wallet: { ...wallet, splittingEnabled: true } });
  const field = () => screen.getByRole("textbox", { name: "Parte padrão de Bia Lima (%)" }) as HTMLInputElement;
  const saveChip = () => screen.queryByRole("button", { name: "Salvar" });

  it("shows the field with the stored share formatted, and the hint", () => {
    open({ ...bia, defaultShareBp: 3333 }, ctx());
    expect(field().value).toBe("33,33");
    expect(field().placeholder).toBe("igual");
    expect(screen.getByText(/Em branco, a divisão é igual/)).toBeTruthy();
    expect(saveChip()).toBeNull();
  });

  it("is empty when no share is set", () => {
    open(bia, ctx());
    expect(field().value).toBe("");
  });
  it("hidden without splitting", () => {
    open(bia);
    expect(screen.queryByLabelText(/Parte padrão/)).toBeNull();
  });
  it("hidden when the member is not in the default split", () => {
    open({ ...bia, splitsByDefault: false }, ctx());
    expect(screen.queryByLabelText(/Parte padrão/)).toBeNull();
  });
  it("hidden for a non-owner", () => {
    open(bia, { ...asBia(), wallet: { ...wallet, role: "editor", splittingEnabled: true } });
    expect(screen.queryByLabelText(/Parte padrão/)).toBeNull();
  });

  it.each([["60", 6000], ["33,33", 3333], ["33.33", 3333], ["", 0], ["0", 0]])("saving %j sends %i", async (text, bp) => {
    const fetch = serve();
    const { user } = open({ ...bia, defaultShareBp: 5000 }, ctx());
    await user.clear(field());
    if (text) await user.type(field(), text);
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/members/2");
    expect(bodyAt(fetch)).toEqual({ defaultShareBp: bp });
    await waitFor(() => expect(saveChip()).toBeNull());
  });

  it.each(["-5", "abc", "101", "33,333"])("refuses %j with an inline error and sends nothing", async (text) => {
    const fetch = serve();
    const { user } = open(bia, ctx());
    await user.type(field(), text);
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText(/Informe uma porcentagem/)).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("shows the server's field error", async () => {
    mockFetch((_u, init) => (init.method === "PATCH" ? { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { defaultShareBp: "parte inválida" } } } } : { body: [] }));
    const { user } = open(bia, ctx());
    await user.type(field(), "50");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("parte inválida")).toBeTruthy();
  });
});
