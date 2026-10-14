import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { PaymentMethod } from "../api/types";
import { bodyOf, mockFetch, paymentMethods, renderApp } from "../test/fixtures";
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
});
