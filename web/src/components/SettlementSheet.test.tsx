import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Settlement } from "../api/types";
import { today } from "../lib/dates";
import { appCtx, bodyOf, mockFetch, renderApp } from "../test/fixtures";
import { SettlementSheet } from "./SettlementSheet";

const past: Settlement = { id: 3, fromUserId: 2, toUserId: 1, amountCents: 5000, date: "2026-09-20", note: "Pix do aluguel", recurrenceId: null, createdBy: 1 };
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const serve = (status = 200, body: unknown = {}) => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { status, body } : { body: [] }));
const row = (label: string) => screen.getByRole("button", { name: new RegExp(`^${label}`) });

function open(s: Settlement | "new" | null) {
  const onClose = vi.fn();
  renderApp(<SettlementSheet settlement={s} onClose={onClose} />, appCtx());
  return { onClose, user: userEvent.setup() };
}

describe("SettlementSheet: new", () => {
  it("is closed without a settlement", () => {
    open(null);
    expect(screen.queryByLabelText("Valor do acerto")).toBeNull();
  });

  it("starts with the other person paying you, today", () => {
    open("new");
    expect(row("Quem pagou").textContent).toContain("Bia");
    expect(row("Quem recebeu").textContent).toContain("Ana");
    expect((screen.getByLabelText("Data do acerto") as HTMLInputElement).value).toBe(today());
  });

  it("registers with the amount, the people, the date and a trimmed note", async () => {
    const fetch = serve(201);
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Valor do acerto"), "75,50");
    await user.type(screen.getByLabelText("Observação do acerto"), "  Pix  ");
    await user.click(screen.getByRole("button", { name: "Registrar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements");
    expect(writes(fetch)[0][1]!.method).toBe("POST");
    expect(bodyAt(fetch)).toEqual({ fromUserId: 2, toUserId: 1, amountCents: 7550, date: today(), note: "Pix" });
  });

  it.each(["", "0", "abc"])("the amount %j is refused before anything is sent", async (typed) => {
    const fetch = serve();
    const { user } = open("new");
    if (typed) await user.type(screen.getByLabelText("Valor do acerto"), typed);
    await user.click(screen.getByRole("button", { name: "Registrar" }));
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("the same person cannot pay and receive", async () => {
    const fetch = serve();
    const { user } = open("new");
    await user.type(screen.getByLabelText("Valor do acerto"), "10");
    await user.click(row("Quem pagou"));
    await user.click(screen.getByRole("button", { name: "Ana (você)" }));
    await user.click(screen.getByRole("button", { name: "Registrar" }));
    expect(await screen.findByText(/Escolha pessoas diferentes/)).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("shows the error of the API and stays open", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { date: "data inválida" } } } } : { body: [] }));
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Valor do acerto"), "10");
    fireEvent.change(screen.getByLabelText("Data do acerto"), { target: { value: "2026-02-30" } });
    await user.click(screen.getByRole("button", { name: "Registrar" }));
    expect(await screen.findByText("data inválida")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("SettlementSheet: existing", () => {
  it("is prefilled and saves with PUT, keeping the date and the note", async () => {
    const fetch = serve();
    const { user, onClose } = open(past);
    const amount = screen.getByLabelText("Valor do acerto") as HTMLInputElement;
    expect(amount.value).toBe("50,00");
    await user.clear(amount);
    await user.type(amount, "60,00");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements/3");
    expect(writes(fetch)[0][1]!.method).toBe("PUT");
    expect(bodyAt(fetch)).toEqual({ fromUserId: 2, toUserId: 1, amountCents: 6000, date: "2026-09-20", note: "Pix do aluguel" });
  });

  it("deletes only after confirming, and a double click does not confirm", async () => {
    const fetch = serve(204, undefined);
    const { user, onClose } = open(past);
    await user.dblClick(screen.getByRole("button", { name: "Excluir acerto" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements/3");
  });
});

describe("SettlementSheet: focus", () => {
  it("coming back from a picker puts focus on the row that opened it", async () => {
    serve();
    const { user } = open("new");
    await user.click(row("Quem pagou"));
    await user.click(screen.getByRole("button", { name: "Ana (você)" }));
    expect(document.activeElement).toBe(row("Quem pagou"));
  });
});
