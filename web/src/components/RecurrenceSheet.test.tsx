import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Recurrence } from "../api/types";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../test/fixtures";
import { RecurrenceSheet, recurrenceBody } from "./RecurrenceSheet";

const netflix: Recurrence = {
  id: 5, kind: "expense", amountCents: 5590, description: "Netflix", categoryId: 1, paymentMethodId: 2, paidByUserId: 1,
  toUserId: null, dayOfMonth: 5, startDate: "2026-01-05", endDate: null, enabled: true, nextDate: "2026-11-05",
  splits: [{ userId: 1, shareBp: 6000, amountCents: 3354 }, { userId: 2, shareBp: 4000, amountCents: 2236 }],
};
const settlement: Recurrence = { ...netflix, id: 6, kind: "settlement", categoryId: null, paymentMethodId: null, toUserId: 2, description: "", splits: [] };

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const serve = () => mockFetch({ body: {} });
function open(r: Recurrence | null = netflix, ctx = appCtx()) {
  const onClose = vi.fn();
  renderApp(<RecurrenceSheet recurrence={r} onClose={onClose} />, ctx);
  return { onClose, user: userEvent.setup() };
}

describe("recurrenceBody", () => {
  it("is the whole object for the PUT, with the overrides applied", () => {
    expect(recurrenceBody(netflix, { enabled: false, amountCents: 100 }, false)).toEqual({
      kind: "expense", amountCents: 100, description: "Netflix", categoryId: 1, paymentMethodId: 2, paidByUserId: 1, toUserId: null,
      dayOfMonth: 5, startDate: "2026-01-05", endDate: null, enabled: false,
    });
  });

  it("sends the split parts (without amounts) only for expenses on a wallet that splits", () => {
    expect(recurrenceBody(netflix, {}, true).splits).toEqual([{ userId: 1, shareBp: 6000 }, { userId: 2, shareBp: 4000 }]);
    expect(recurrenceBody(netflix, {}, false)).not.toHaveProperty("splits");
    expect(recurrenceBody({ ...netflix, kind: "income" }, {}, true)).not.toHaveProperty("splits");
    expect(recurrenceBody(settlement, {}, true)).not.toHaveProperty("splits");
  });
});

describe("RecurrenceSheet", () => {
  it("is closed without a recurrence", () => {
    open(null);
    expect(screen.queryByLabelText("Valor")).toBeNull();
  });

  it("opens prefilled", () => {
    open();
    expect((screen.getByLabelText("Valor") as HTMLInputElement).value).toBe("55,90");
    expect((screen.getByLabelText("Descrição") as HTMLInputElement).value).toBe("Netflix");
    expect((screen.getByLabelText("Dia do mês") as HTMLInputElement).value).toBe("5");
  });

  it("refuses a day outside 1-31 and a zero amount before sending", async () => {
    const fetch = serve();
    const { user } = open();
    await user.clear(screen.getByLabelText("Dia do mês"));
    await user.type(screen.getByLabelText("Dia do mês"), "40");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("O dia deve estar entre 1 e 31.")).toBeTruthy();
    await user.clear(screen.getByLabelText("Dia do mês"));
    await user.type(screen.getByLabelText("Dia do mês"), "0");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(screen.getByText("O dia deve estar entre 1 e 31.")).toBeTruthy();
    await user.clear(screen.getByLabelText("Dia do mês"));
    await user.type(screen.getByLabelText("Dia do mês"), "10");
    await user.clear(screen.getByLabelText("Valor"));
    await user.type(screen.getByLabelText("Valor"), "0");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("saves the whole object by PUT with the new amount and day, then closes", async () => {
    const fetch = serve();
    const { user, onClose } = open();
    await user.clear(screen.getByLabelText("Dia do mês"));
    await user.type(screen.getByLabelText("Dia do mês"), "10");
    await user.clear(screen.getByLabelText("Valor"));
    await user.type(screen.getByLabelText("Valor"), "59,90");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    const [url, init] = writes(fetch)[0];
    expect(url).toBe("/api/wallets/1/recurrences/5");
    expect(init!.method).toBe("PUT");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toMatchObject({ amountCents: 5990, dayOfMonth: 10, categoryId: 1, paymentMethodId: 2, startDate: "2026-01-05", enabled: true });
  });

  it("only sends the split parts when the wallet splits expenses", async () => {
    const fetch = serve();
    const { user } = open(netflix, appCtx({ wallet: { ...wallet, splittingEnabled: true } }));
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0])).splits).toEqual([{ userId: 1, shareBp: 6000 }, { userId: 2, shareBp: 4000 }]);
  });

  it("deletes only after confirming, then closes", async () => {
    const fetch = serve();
    const { user, onClose } = open();
    await user.click(screen.getByRole("button", { name: "Excluir recorrência" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/recurrences/5");
  });

  it("an end date is sent, and clearing it sends null", async () => {
    const fetch = serve();
    const { user } = open({ ...netflix, endDate: "2027-01-05" });
    expect((screen.getByLabelText("Data final") as HTMLInputElement).value).toBe("2027-01-05");
    await user.clear(screen.getByLabelText("Data final"));
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0])).endDate).toBeNull();
  });

  it("a settlement (no category) opens and saves without asking for one", async () => {
    const fetch = serve();
    const { user, onClose } = open(settlement);
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toMatchObject({ kind: "settlement", categoryId: null, toUserId: 2 });
  });

  it("shows the server's error and keeps the sheet open", async () => {
    mockFetch((_u, init) => (init.method === "PUT" ? { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { dayOfMonth: "dia inválido" } } } } : { body: {} }));
    const { user, onClose } = open();
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("dia inválido")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });
});
