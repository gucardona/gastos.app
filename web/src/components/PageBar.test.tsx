import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, renderApp, wallet } from "../test/fixtures";
import { MonthSwitcher, PageBar } from "./PageBar";

describe("PageBar", () => {
  it("mostra a carteira ativa, com quem ela é compartilhada, e a inicial do usuário", () => {
    renderApp(<PageBar />);
    expect((screen.getByLabelText("Carteira") as HTMLSelectElement).value).toBe("1");
    expect(screen.getByText("com Bia")).toBeTruthy();
    const avatar = screen.getByRole("link", { name: "Ajustes" });
    expect(avatar.textContent).toBe("A");
    expect(avatar.getAttribute("href")).toBe("/ajustes");
  });

  it("carteira só sua não mostra 'com ninguém'", () => {
    renderApp(<PageBar />, appCtx({ members: [appCtx().members[0]] }));
    expect(screen.queryByText(/^com /)).toBeNull();
  });

  it("troca de carteira pelo seletor", async () => {
    const ctx = appCtx({ wallets: [wallet, { ...wallet, id: 7, name: "Viagem" }] });
    renderApp(<PageBar />, ctx);
    await userEvent.selectOptions(screen.getByLabelText("Carteira"), "7");
    expect(ctx.setWalletId).toHaveBeenCalledWith(7);
  });
});

describe("MonthSwitcher", () => {
  it("mostra o mês por extenso e navega para os vizinhos", async () => {
    const ctx = appCtx({ month: "2026-01" });
    const user = userEvent.setup();
    renderApp(<MonthSwitcher />, ctx);
    expect(screen.getByText("janeiro de 2026")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Mês anterior" }));
    expect(ctx.setMonth).toHaveBeenLastCalledWith("2025-12"); // vira o ano
    await user.click(screen.getByRole("button", { name: "Próximo mês" }));
    expect(ctx.setMonth).toHaveBeenLastCalledWith("2026-02");
  });
});
