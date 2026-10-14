import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, mockFetch, paymentMethods, renderApp, wallet } from "../../test/fixtures";
import { PaymentsSection } from "./PaymentsSection";

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
});
