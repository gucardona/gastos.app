import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { PaymentMethod } from "../api/types";
import { appCtx, bodyOf, members, mockFetch, paymentMethods, renderApp } from "../test/fixtures";
import { PaymentSheet } from "./PaymentSheet";

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const serve = (status = 200, body: unknown = {}) => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { status, body } : { body: [] }));

function open(method: PaymentMethod | "new" | null) {
  const onClose = vi.fn();
  renderApp(<PaymentSheet method={method} onClose={onClose} />);
  return { onClose, user: userEvent.setup() };
}

describe("PaymentSheet", () => {
  const pix = paymentMethods[0];

  it("is closed without a method", () => {
    open(null);
    expect(screen.queryByLabelText("Nome da forma de pagamento")).toBeNull();
  });

  it("creates a payment method with a name and an emoji", async () => {
    const fetch = serve(201);
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Nome da forma de pagamento"), "Cheque");
    await user.click(screen.getByRole("button", { name: "Dinheiro" }));
    await user.click(screen.getByRole("button", { name: "🧾" }));
    await user.click(screen.getByRole("button", { name: "Criar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/payment-methods");
    expect(bodyAt(fetch)).toEqual({ name: "Cheque", icon: "🧾" });
  });

  it("shows the error of the API and stays open", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 409, body: { error: { code: "conflict", message: "Já existe uma forma de pagamento com esse nome" } } } : { body: [] }));
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Nome da forma de pagamento"), "Pix");
    await user.click(screen.getByRole("button", { name: "Criar" }));
    expect(await screen.findByText("Já existe uma forma de pagamento com esse nome")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("edits by PUT with the whole object, keeping the order and the archived flag", async () => {
    const fetch = serve();
    const { user, onClose } = open(pix);
    expect((screen.getByLabelText("Nome da forma de pagamento") as HTMLInputElement).value).toBe("Pix");
    await user.clear(screen.getByLabelText("Nome da forma de pagamento"));
    await user.type(screen.getByLabelText("Nome da forma de pagamento"), "Pix do banco");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/payment-methods/1");
    expect(bodyAt(fetch)).toEqual({ name: "Pix do banco", icon: "⚡", sortOrder: 0, archived: false });
  });

  it("deletes only after confirming", async () => {
    const fetch = serve(204, undefined);
    const { user, onClose } = open(pix);
    await user.click(screen.getByRole("button", { name: "Excluir forma de pagamento" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
  });

  it("an archived method offers to restore it", async () => {
    const fetch = serve();
    const { user } = open({ ...pix, archived: true });
    expect(screen.queryByRole("button", { name: "Excluir forma de pagamento" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Restaurar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toMatchObject({ archived: false, name: "Pix" });
  });

  it("archives with a PUT that flips archived", async () => {
    const fetch = serve();
    const { user } = open(pix);
    await user.click(screen.getByRole("button", { name: "Arquivar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toMatchObject({ archived: true });
  });

  it("'Padrão ao lançar' makes it the method my new transactions start with, and turning it off goes back to the first", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<PaymentSheet method={pix} onClose={vi.fn()} />);
    await user.click(screen.getByRole("switch", { name: "Padrão ao lançar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/default-payment-method");
    expect(writes(fetch)[0][1]!.method).toBe("PUT");
    expect(bodyAt(fetch)).toEqual({ paymentMethodId: 1 });
    cleanup();

    const again = serve();
    renderApp(<PaymentSheet method={pix} onClose={vi.fn()} />, appCtx({ members: members.map((m) => (m.userId === 1 ? { ...m, defaultPaymentMethodId: 1 } : m)) }));
    const on = screen.getByRole("switch", { name: "Padrão ao lançar" });
    expect(on.getAttribute("aria-checked")).toBe("true");
    await user.click(on);
    await waitFor(() => expect(writes(again)).toHaveLength(1));
    expect(bodyAt(again)).toEqual({ paymentMethodId: null });
  });

  it("a new or archived method has no 'Padrão ao lançar'", () => {
    open("new");
    expect(screen.queryByRole("switch", { name: "Padrão ao lançar" })).toBeNull();
    cleanup();
    open({ ...pix, archived: true });
    expect(screen.queryByRole("switch", { name: "Padrão ao lançar" })).toBeNull();
  });
});

describe("PaymentSheet as a credit card", () => {
  const card: PaymentMethod = {
    id: 3, name: "Itaú Black", icon: "💳", sortOrder: 2, archived: false,
    card: { closingDay: 20, dueDay: 27, issuer: "Itaú", cardForm: "virtual", last4: "1234", bestPurchaseDay: 21 },
  };
  const openCard = (method: PaymentMethod | "new") => {
    const onClose = vi.fn();
    renderApp(<PaymentSheet method={method} card onClose={onClose} />);
    return { onClose, user: userEvent.setup() };
  };

  it("a plain payment method has no card switch nor card fields", () => {
    serve();
    open("new");
    expect(screen.queryByRole("switch", { name: "Cartão de crédito" })).toBeNull();
    expect(screen.queryByLabelText("Compras até o dia")).toBeNull();
  });

  it("a new card shows its fields right away, and the best purchase day", async () => {
    serve(201);
    const { user } = openCard("new");
    expect(screen.getByRole("heading", { name: "Novo cartão" })).toBeTruthy();
    expect(screen.queryByRole("switch")).toBeNull();
    for (const f of ["Nome do cartão", "Banco ou bandeira", "Final do cartão", "Compras até o dia", "Vence no dia"]) expect(screen.getByLabelText(f)).toBeTruthy();
    expect(screen.getByRole("group", { name: "Tipo do cartão" })).toBeTruthy();
    await user.type(screen.getByLabelText("Compras até o dia"), "20");
    expect(screen.getByText("Melhor dia de compra: 21")).toBeTruthy();
    await user.clear(screen.getByLabelText("Compras até o dia"));
    await user.type(screen.getByLabelText("Compras até o dia"), "31");
    expect(screen.getByText("Melhor dia de compra: 1")).toBeTruthy();
  });

  it("creates a card with its fields", async () => {
    const fetch = serve(201);
    const { user, onClose } = openCard("new");
    await user.type(screen.getByLabelText("Nome do cartão"), "Itaú Black");
    await user.type(screen.getByLabelText("Banco ou bandeira"), "Itaú");
    await user.click(screen.getByRole("button", { name: "Virtual" }));
    await user.type(screen.getByLabelText("Final do cartão"), "1234");
    await user.type(screen.getByLabelText("Compras até o dia"), "20");
    await user.type(screen.getByLabelText("Vence no dia"), "27");
    await user.click(screen.getByRole("button", { name: "Criar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(bodyAt(fetch)).toEqual({ name: "Itaú Black", icon: "💳", card: { closingDay: 20, dueDay: 27, issuer: "Itaú", cardForm: "virtual", last4: "1234" } });
  });

  it("an existing card opens as a card, with its values", () => {
    serve();
    open(card); // a card opens as one even without the card prop
    expect(screen.getByRole("heading", { name: "Editar cartão" })).toBeTruthy();
    expect((screen.getByLabelText("Compras até o dia") as HTMLInputElement).value).toBe("20");
    expect((screen.getByLabelText("Final do cartão") as HTMLInputElement).value).toBe("1234");
  });

  it("archiving a card sends no card fields, so the server keeps them", async () => {
    const fetch = serve();
    const { user } = openCard(card);
    await user.click(screen.getByRole("button", { name: "Arquivar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).not.toHaveProperty("card");
  });
});
