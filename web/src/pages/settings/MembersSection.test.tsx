import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../../test/fixtures";
import { MembersSection } from "./MembersSection";

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const ok = () => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: [] }));
const asBia = () => appCtx({ user: { id: 2, name: "Bia Lima", email: "bia@example.com" }, wallet: { ...wallet, role: "editor" } });

describe("People screen", () => {
  it("lists everyone with the email and the permission", () => {
    renderApp(<MembersSection />);
    const ana = screen.getByRole("button", { name: /Ana Souza/ });
    expect(ana.textContent).toContain("(você)");
    expect(ana.textContent).toContain("Administra");
    expect(screen.getByRole("button", { name: /Bia Lima/ }).textContent).toContain("bia@example.com");
  });

  it("invites by email with the chosen permission", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.click(screen.getByRole("button", { name: "Convidar alguém" }));
    await user.type(screen.getByLabelText("E-mail do convidado"), "caio@example.com");
    await user.click(screen.getByRole("button", { name: "Só leitura" }));
    await user.click(screen.getByRole("button", { name: "Convidar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/members");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ email: "caio@example.com", role: "reader" });
  });

  it("invites as an editor by default, and cannot send an empty email", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.click(screen.getByRole("button", { name: "Convidar alguém" }));
    expect((screen.getByRole("button", { name: "Convidar" }) as HTMLButtonElement).disabled).toBe(true);
    await user.type(screen.getByLabelText("E-mail do convidado"), "caio@example.com");
    await user.click(screen.getByRole("button", { name: "Convidar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0])).role).toBe("editor");
  });

  it("shows the error when the email has no account, keeping the sheet open", async () => {
    mockFetch({ status: 404, body: { error: { code: "not_found", message: "Usuário não encontrado" } } });
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.click(screen.getByRole("button", { name: "Convidar alguém" }));
    await user.type(screen.getByLabelText("E-mail do convidado"), "x@example.com");
    await user.click(screen.getByRole("button", { name: "Convidar" }));
    expect(await screen.findByText("Usuário não encontrado")).toBeTruthy();
    expect(screen.getByLabelText("E-mail do convidado")).toBeTruthy();
  });

  it("tapping another person opens their permissions", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.click(screen.getByRole("button", { name: /Bia Lima/ }));
    expect(screen.getByRole("group", { name: "Permissão" })).toBeTruthy();
  });

  it("someone who is not the owner cannot invite, and only their own row opens (to leave)", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<MembersSection />, asBia());
    expect(screen.queryByRole("button", { name: "Convidar alguém" })).toBeNull();
    expect(screen.queryByRole("button", { name: /Ana Souza/ })).toBeNull();
    await user.click(screen.getByRole("button", { name: /Bia Lima/ }));
    expect(screen.getByRole("button", { name: "Sair da carteira" })).toBeTruthy();
  });
});
