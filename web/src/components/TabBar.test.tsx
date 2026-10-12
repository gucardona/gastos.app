import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { appCtx, renderApp, wallet } from "../test/fixtures";
import { TabBar } from "./TabBar";

const nav = () => screen.getByRole("navigation", { name: "Navegação" });
const labels = () => within(nav()).getAllByRole("link").map((a) => a.textContent?.replace(/\d+\+?$/, ""));

describe("TabBar", () => {
  it("links the destinations in order", () => {
    renderApp(<TabBar pending={0} onAdd={vi.fn()} />);
    expect(labels()).toEqual(["Visão", "Lançamentos", "Caixa"]);
    expect(within(nav()).getByRole("link", { name: "Lançamentos" }).getAttribute("href")).toBe("/lancamentos");
  });

  it("shows Divisão only when the wallet splits expenses", () => {
    renderApp(<TabBar pending={0} onAdd={vi.fn()} />, appCtx({ wallet: { ...wallet, splittingEnabled: true } }));
    expect(labels()).toEqual(["Visão", "Lançamentos", "Caixa", "Divisão"]);
  });

  it("the + button opens Novo lançamento", async () => {
    const onAdd = vi.fn();
    renderApp(<TabBar pending={0} onAdd={onAdd} />);
    await userEvent.click(screen.getByRole("button", { name: "Novo lançamento" }));
    expect(onAdd).toHaveBeenCalledTimes(1);
  });

  it("a reader has no + button", () => {
    renderApp(<TabBar pending={0} onAdd={vi.fn()} />, appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } }));
    expect(screen.queryByRole("button", { name: "Novo lançamento" })).toBeNull();
  });

  it("shows how many lines wait in the Caixa, and nothing when none do", () => {
    const { unmount } = renderApp(<TabBar pending={4} onAdd={vi.fn()} />);
    expect(within(nav()).getByRole("link", { name: /Caixa/ }).textContent).toContain("4");
    unmount();
    renderApp(<TabBar pending={0} onAdd={vi.fn()} />);
    expect(within(nav()).getByRole("link", { name: "Caixa" }).textContent).toBe("Caixa");
  });

  it("caps the counter at 99+", () => {
    renderApp(<TabBar pending={250} onAdd={vi.fn()} />);
    expect(within(nav()).getByRole("link", { name: /Caixa/ }).textContent).toContain("99+");
  });

  it("marks only the current destination", () => {
    renderApp(<TabBar pending={0} onAdd={vi.fn()} />, appCtx(), "/lancamentos");
    const current = within(nav()).getAllByRole("link").filter((a) => a.getAttribute("aria-current") === "page");
    expect(current.map((a) => a.getAttribute("href"))).toEqual(["/lancamentos"]);
  });
});
