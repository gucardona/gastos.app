import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, mockFetch, paymentMethods, renderApp, wallet } from "../../test/fixtures";
import { PaymentsSection } from "./PaymentsSection";

const itau = { id: 9, name: "Itaú Black", icon: "💳", sortOrder: 3, archived: false, card: { closingDay: 20, dueDay: 27, issuer: "", cardForm: "" as const, last4: "1234", bestPurchaseDay: 21 } };

describe("Payment methods screen", () => {
  it("lists the methods, and tapping one opens it for editing", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<PaymentsSection />);
    expect(screen.getByRole("button", { name: /Crédito/ })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Pix/ }));
    expect((screen.getByLabelText("Nome da forma de pagamento") as HTMLInputElement).value).toBe("Pix");
  });

  it("'Nova forma de pagamento' opens an empty sheet", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<PaymentsSection />);
    await user.click(screen.getByRole("button", { name: "Nova forma de pagamento" }));
    expect((screen.getByLabelText("Nome da forma de pagamento") as HTMLInputElement).value).toBe("");
    expect(screen.getByRole("button", { name: "Criar" })).toBeTruthy();
  });

  it("archived methods are behind a toggle, and open to be restored", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<PaymentsSection />, appCtx({ paymentMethods: [paymentMethods[0], { ...paymentMethods[1], archived: true }] }));
    expect(screen.queryByText("Crédito")).toBeNull();
    await user.click(screen.getByRole("button", { name: /Mostrar arquivadas \(1\)/ }));
    await user.click(screen.getByRole("button", { name: /Crédito/ }));
    expect(screen.getByRole("button", { name: "Restaurar" })).toBeTruthy();
  });

  it("a reader sees the methods but cannot create or open them", () => {
    renderApp(<PaymentsSection />, appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } }));
    expect(screen.getByText("Pix")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Nova forma de pagamento" })).toBeNull();
    expect(screen.queryByRole("button", { name: /Pix/ })).toBeNull();
  });

  it("a card shows among the methods but is managed in Cartões de crédito", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<PaymentsSection />, appCtx({ paymentMethods: [...paymentMethods, itau] }), "/ajustes/pagamentos");
    const row = screen.getByRole("link", { name: /Itaú Black/ });
    expect(row.textContent).toContain("Em Cartões de crédito");
    expect(row.getAttribute("href")).toBe("/ajustes/cartoes");
    await user.click(screen.getByRole("button", { name: "Nova forma de pagamento" }));
    expect(screen.queryByRole("switch", { name: "Cartão de crédito" })).toBeNull();
  });
});

describe("Credit cards screen", () => {
  it("lists only the cards with their days, and creates a new card", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<PaymentsSection cards />, appCtx({ paymentMethods: [...paymentMethods, itau] }));
    expect(screen.getByRole("button", { name: /Itaú Black/ }).textContent).toContain("fecha 20 · vence 27 · final 1234");
    expect(screen.queryByText("Pix")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Novo cartão" }));
    expect(screen.getByRole("heading", { name: "Novo cartão" })).toBeTruthy();
  });
});
