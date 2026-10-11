import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { appCtx, bodyOf, mockFetch, renderApp, tx, wallet } from "../test/fixtures";
import { today } from "../lib/dates";
import { AddSheet } from "./AddSheet";

const save = () => screen.getByRole("button", { name: "Salvar" });

function open(state: { tx?: ReturnType<typeof tx> } = {}, ctx = appCtx()) {
  const onClose = vi.fn();
  renderApp(<AddSheet state={state} onClose={onClose} />, ctx);
  return { onClose, user: userEvent.setup() };
}

async function fill(user: ReturnType<typeof userEvent.setup>, amount = "48,90", description = "Mercado do bairro") {
  await user.type(screen.getByRole("textbox", { name: "Valor" }), amount);
  await user.type(screen.getByRole("textbox", { name: "Descrição" }), description);
}

describe("AddSheet: new transaction", () => {
  it("does not submit and shows what is missing when the form is empty", async () => {
    const fetch = mockFetch();
    const { user } = open();
    await user.click(save());
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(screen.getByText("Descreva o lançamento.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("creates the expense with the default payment method and closes", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user, onClose } = open();
    await fill(user);
    await user.click(screen.getByRole("button", { name: /Restaurante/ }));
    await user.click(save());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/transactions");
    expect(fetch.mock.calls[0][1]!.method).toBe("POST");
    expect(bodyOf(fetch)).toEqual({
      kind: "expense", amountCents: 4890, description: "Mercado do bairro", categoryId: 2,
      paymentMethodId: 1, paidByUserId: 1, date: today(),
    });
  });

  it("switches to income and uses the income categories", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open();
    await user.click(screen.getByRole("button", { name: "Entrada" }));
    expect(screen.queryByRole("button", { name: /Mercado/ })).toBeNull();
    await fill(user, "8500", "Salário de outubro");
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch)).toMatchObject({ kind: "income", amountCents: 850000, categoryId: 3 });
  });

  it("'Salvar e lançar outro' keeps the sheet open and clears amount and description", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user, onClose } = open();
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Salvar e lançar outro" }));
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    await waitFor(() => expect((screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement).value).toBe(""));
    expect((screen.getByRole("textbox", { name: "Descrição" }) as HTMLInputElement).value).toBe("");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows the field error returned by the API", async () => {
    mockFetch({ status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { categoryId: "categoria arquivada" } } } });
    const { user, onClose } = open();
    await fill(user);
    await user.click(save());
    expect(await screen.findByText("categoria arquivada")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows the general message when the API fails without fields", async () => {
    mockFetch({ status: 403, body: { error: { code: "forbidden", message: "Sem permissão para esta ação" } } });
    const { user } = open();
    await fill(user);
    await user.click(save());
    expect(await screen.findByText("Sem permissão para esta ação")).toBeTruthy();
  });

  it("'Repetir todo mês' creates a recurrence starting at the date", async () => {
    const fetch = mockFetch({ status: 201, body: {} });
    const { user } = open();
    await fill(user);
    await user.click(screen.getByRole("switch", { name: "Repetir todo mês" }));
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/recurrences");
    expect(bodyOf(fetch)).toMatchObject({ dayOfMonth: Number(today().slice(8)), startDate: today(), amountCents: 4890 });
  });
});

describe("AddSheet: splitting", () => {
  const splitting = () => appCtx({ wallet: { ...wallet, splittingEnabled: true } });

  it("shows no splitting on a wallet without splitting", () => {
    open();
    expect(screen.queryByRole("button", { name: "Ajustar" })).toBeNull();
  });

  it("rejects percentages that do not add up to 100% and submits when they do", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user, onClose } = open({}, splitting());
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Ajustar" }));

    const bia = screen.getByRole("textbox", { name: "Porcentagem de Bia Lima" });
    await user.clear(bia);
    await user.type(bia, "40");
    await user.click(save());
    expect(await screen.findByText("As partes precisam somar 100%.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();

    await user.clear(screen.getByRole("textbox", { name: "Porcentagem de Ana Souza" }));
    await user.type(screen.getByRole("textbox", { name: "Porcentagem de Ana Souza" }), "60");
    await user.click(save());
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(bodyOf(fetch).splits).toEqual([
      { userId: 1, shareBp: 6000 },
      { userId: 2, shareBp: 4000 },
    ]);
  });

  it("the default split sends no splits (the server splits equally)", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open({}, splitting());
    await fill(user);
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch)).not.toHaveProperty("splits");
  });

  it("lets the user choose who paid", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open({}, splitting());
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Bia" }));
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch).paidByUserId).toBe(2);
  });
});

describe("AddSheet: editing", () => {
  it("is prefilled with the transaction and saves with PUT", async () => {
    const fetch = mockFetch({ body: tx() });
    const { user, onClose } = open({ tx: tx({ amountCents: 12990, description: "Feira" }) });
    expect((screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement).value).toBe("129,90");
    await user.clear(screen.getByRole("textbox", { name: "Descrição" }));
    await user.type(screen.getByRole("textbox", { name: "Descrição" }), "Feira da semana");
    await user.click(save());
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/transactions/10");
    expect(fetch.mock.calls[0][1]!.method).toBe("PUT");
    expect(bodyOf(fetch)).toMatchObject({ amountCents: 12990, description: "Feira da semana" });
  });

  it("deletes only after confirming", async () => {
    const fetch = mockFetch({ status: 204 });
    const { user, onClose } = open({ tx: tx() });
    await user.click(screen.getByRole("button", { name: "Excluir lançamento" }));
    expect(fetch).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(fetch.mock.calls[0][1]!.method).toBe("DELETE");
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/transactions/10");
  });

  it("does not offer 'Repetir todo mês' or 'lançar outro' when editing", () => {
    open({ tx: tx() });
    expect(screen.queryByRole("switch", { name: "Repetir todo mês" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Salvar e lançar outro" })).toBeNull();
  });
});
