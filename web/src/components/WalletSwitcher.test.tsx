import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, renderApp, wallet } from "../test/fixtures";
import { WalletSwitcher } from "./WalletSwitcher";

const two = () => appCtx({ wallets: [wallet, { ...wallet, id: 7, name: "Viagem" }] });

describe("WalletSwitcher", () => {
  it("shows the active wallet and who it is shared with", () => {
    renderApp(<WalletSwitcher />, two());
    expect(screen.getByRole("button", { name: /Carteira/ }).textContent).toContain("Casa");
    expect(screen.getByText("com Bia")).toBeTruthy();
  });

  it("opens a list of the wallets, marks the current one and switches on tap", async () => {
    const ctx = two();
    renderApp(<WalletSwitcher />, ctx);
    await userEvent.click(screen.getByRole("button", { name: /Carteira/ }));
    const sheet = screen.getByRole("dialog");
    expect(within(sheet).getByRole("button", { name: /Casa/ }).getAttribute("aria-pressed")).toBe("true");
    await userEvent.click(within(sheet).getByRole("button", { name: /Viagem/ }));
    expect(ctx.setWalletId).toHaveBeenCalledWith(7);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("tells apart a wallet that someone else owns when the names are the same", async () => {
    const ctx = appCtx({ wallets: [wallet, { ...wallet, id: 7, role: "editor", ownerName: "Bia Lima" }] });
    renderApp(<WalletSwitcher />, ctx);
    await userEvent.click(screen.getByRole("button", { name: /Carteira/ }));
    const sheet = screen.getByRole("dialog");
    expect(within(sheet).getByRole("button", { name: "Casa" })).toBeTruthy(); // yours: just the name
    expect(within(sheet).getByRole("button", { name: "Casa, de Bia" })).toBeTruthy();
  });

  it("with a single wallet it is just a title, nothing to open", () => {
    renderApp(<WalletSwitcher />);
    expect(screen.queryByRole("button", { name: /Carteira/ })).toBeNull();
    expect(screen.getByText("Casa")).toBeTruthy();
  });

  it("a wallet that is only yours shows no 'com ...' label", () => {
    renderApp(<WalletSwitcher />, appCtx({ members: [appCtx().members[0]] }));
    expect(screen.queryByText(/^com /)).toBeNull();
  });
});
