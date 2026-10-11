import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, renderApp, wallet } from "../test/fixtures";
import { MonthSwitcher, PageBar } from "./PageBar";

describe("PageBar", () => {
  it("shows the active wallet, who it is shared with, and the user's initial", () => {
    renderApp(<PageBar />);
    expect((screen.getByLabelText("Carteira") as HTMLSelectElement).value).toBe("1");
    expect(screen.getByText("com Bia")).toBeTruthy();
    const avatar = screen.getByRole("link", { name: "Ajustes" });
    expect(avatar.textContent).toBe("A");
    expect(avatar.getAttribute("href")).toBe("/ajustes");
  });

  it("a wallet that is only yours shows no 'com ...' label", () => {
    renderApp(<PageBar />, appCtx({ members: [appCtx().members[0]] }));
    expect(screen.queryByText(/^com /)).toBeNull();
  });

  it("switches wallet through the selector", async () => {
    const ctx = appCtx({ wallets: [wallet, { ...wallet, id: 7, name: "Viagem" }] });
    renderApp(<PageBar />, ctx);
    await userEvent.selectOptions(screen.getByLabelText("Carteira"), "7");
    expect(ctx.setWalletId).toHaveBeenCalledWith(7);
  });
});

describe("MonthSwitcher", () => {
  it("shows the month spelled out and navigates to the neighbours", async () => {
    const ctx = appCtx({ month: "2026-01" });
    const user = userEvent.setup();
    renderApp(<MonthSwitcher />, ctx);
    expect(screen.getByText("janeiro de 2026")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Mês anterior" }));
    expect(ctx.setMonth).toHaveBeenLastCalledWith("2025-12"); // crosses the year boundary
    await user.click(screen.getByRole("button", { name: "Próximo mês" }));
    expect(ctx.setMonth).toHaveBeenLastCalledWith("2026-02");
  });
});
