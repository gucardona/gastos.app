import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { appCtx, renderApp, wallet } from "../test/fixtures";
import { Sidebar } from "./Sidebar";

const nav = () => screen.getByRole("navigation", { name: "Navegação" });
const labels = () => within(nav()).getAllByRole("link").map((a) => a.textContent?.replace(/\d+\+?$/, ""));

describe("Sidebar", () => {
  it("links the destinations in order, and Divisão only when the wallet splits", () => {
    const { unmount } = renderApp(<Sidebar pending={0} onAdd={vi.fn()} />);
    expect(labels()).toEqual(["Visão", "Lançamentos", "Caixa"]);
    unmount();
    renderApp(<Sidebar pending={0} onAdd={vi.fn()} />, appCtx({ wallet: { ...wallet, splittingEnabled: true } }));
    expect(labels()).toEqual(["Visão", "Lançamentos", "Caixa", "Divisão"]);
  });

  it("shows Chat after Caixa only when the assistant is on", () => {
    const { unmount } = renderApp(<Sidebar pending={0} onAdd={vi.fn()} />, appCtx({ assistantEnabled: true }));
    expect(labels()).toEqual(["Visão", "Lançamentos", "Caixa", "Chat"]);
    expect(within(nav()).getByRole("link", { name: "Chat" }).getAttribute("href")).toBe("/chat");
    unmount();
    renderApp(<Sidebar pending={0} onAdd={vi.fn()} />);
    expect(labels()).not.toContain("Chat");
  });

  it("has no Recorrentes or Ajustes among the destinations", () => {
    renderApp(<Sidebar pending={0} onAdd={vi.fn()} />);
    expect(within(nav()).queryByRole("link", { name: /Recorrentes|Ajustes/ })).toBeNull();
  });

  it("shows the pending counter on Caixa, capped at 99+", () => {
    const { unmount } = renderApp(<Sidebar pending={4} onAdd={vi.fn()} />);
    expect(within(nav()).getByRole("link", { name: /Caixa/ }).textContent).toContain("4");
    unmount();
    renderApp(<Sidebar pending={250} onAdd={vi.fn()} />);
    expect(within(nav()).getByRole("link", { name: /Caixa/ }).textContent).toContain("99+");
  });

  it("marks only the current destination", () => {
    renderApp(<Sidebar pending={0} onAdd={vi.fn()} />, appCtx(), "/caixa");
    const current = within(nav()).getAllByRole("link").filter((a) => a.getAttribute("aria-current") === "page");
    expect(current.map((a) => a.getAttribute("href"))).toEqual(["/caixa"]);
  });

  it("Novo lançamento opens the sheet, and a reader does not get it", async () => {
    const onAdd = vi.fn();
    const { unmount } = renderApp(<Sidebar pending={0} onAdd={onAdd} />);
    await userEvent.click(screen.getByRole("button", { name: "Novo lançamento" }));
    expect(onAdd).toHaveBeenCalledTimes(1);
    unmount();
    renderApp(<Sidebar pending={0} onAdd={onAdd} />, appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } }));
    expect(screen.queryByRole("button", { name: "Novo lançamento" })).toBeNull();
  });

  it("the profile is the one way to Ajustes", () => {
    renderApp(<Sidebar pending={0} onAdd={vi.fn()} />);
    const profile = screen.getByRole("link", { name: "Ajustes" });
    expect(profile.getAttribute("href")).toBe("/ajustes");
    expect(profile.textContent).toContain("Ana Souza");
  });

  it("shows the active wallet and who it is shared with, and switches wallet", async () => {
    const ctx = appCtx({ wallets: [wallet, { ...wallet, id: 7, name: "Viagem" }] });
    renderApp(<Sidebar pending={0} onAdd={vi.fn()} />, ctx);
    expect((screen.getByLabelText("Carteira") as HTMLSelectElement).value).toBe("1");
    expect(screen.getByText("com Bia")).toBeTruthy();
    await userEvent.selectOptions(screen.getByLabelText("Carteira"), "7");
    expect(ctx.setWalletId).toHaveBeenCalledWith(7);
  });
});
