import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Category } from "../api/types";
import { today, yesterday } from "../lib/dates";
import { appCtx, bodyOf, categories, mockFetch, renderApp, tx, wallet } from "../test/fixtures";
import { AddSheet } from "./AddSheet";

type User = ReturnType<typeof userEvent.setup>;

const save = () => screen.getByRole("button", { name: "Salvar" });
const row = (label: string) => screen.getByRole("button", { name: new RegExp(`^${label}`) });

function open(state: { tx?: ReturnType<typeof tx> } = {}, ctx = appCtx()) {
  const onClose = vi.fn();
  renderApp(<AddSheet state={state} onClose={onClose} />, ctx);
  return { onClose, user: userEvent.setup() };
}

async function fill(user: User, amount = "48,90", description = "Mercado do bairro") {
  await user.type(screen.getByRole("textbox", { name: "Valor" }), amount);
  await user.type(screen.getByRole("textbox", { name: "Descrição" }), description);
}

/** Opens the Categoria row and taps a category: the picker closes by itself. */
async function pickCategory(user: User, name: RegExp) {
  await user.click(row("Categoria"));
  await user.click(screen.getByRole("button", { name }));
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
    await pickCategory(user, /Restaurante/);
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
    expect(row("Categoria").textContent).toContain("Salário");
    await user.click(row("Categoria"));
    expect(screen.queryByRole("button", { name: /Mercado/ })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Voltar" }));
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

  it("a description the API refuses (too long) shows its error and keeps the sheet open", async () => {
    mockFetch({ status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { description: "obrigatória, até 200 caracteres" } } } });
    const { user, onClose } = open();
    await fill(user, "10", "x".repeat(201));
    await user.click(save());
    expect(await screen.findByText("obrigatória, até 200 caracteres")).toBeTruthy();
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

describe("AddSheet: amount formats", () => {
  it.each([
    ["48,90", 4890],
    ["1.234,56", 123456],
    ["10", 1000],
  ])("%s is sent as %i cents", async (typed, cents) => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open();
    await fill(user, typed);
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch).amountCents).toBe(cents);
  });

  it.each(["0", "abc"])("%s is refused before anything is sent", async (typed) => {
    const fetch = mockFetch();
    const { user } = open();
    await fill(user, typed);
    await user.click(save());
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
  });
});

describe("AddSheet: pickers", () => {
  it("the Categoria row opens the grid, and choosing returns with the row updated", async () => {
    const { user } = open();
    expect(row("Categoria").textContent).toContain("Mercado");
    await user.click(row("Categoria"));
    expect(screen.getByRole("heading", { name: "Categoria" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Restaurante/ }));
    expect(screen.queryByRole("heading", { name: "Categoria" })).toBeNull();
    expect(row("Categoria").textContent).toContain("Restaurante");
  });

  it("Voltar returns to the form keeping what was typed, with the focus on the row that opened the picker", async () => {
    const { user } = open();
    await fill(user, "12", "Café");
    await user.click(row("Categoria"));
    await user.click(screen.getByRole("button", { name: "Voltar" }));
    expect((screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement).value).toBe("12");
    expect((screen.getByRole("textbox", { name: "Descrição" }) as HTMLInputElement).value).toBe("Café");
    expect(document.activeElement).toBe(row("Categoria"));
  });

  it("Quando: Ontem sends yesterday, and another day typed in the field is sent as is", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open();
    expect(row("Quando").textContent).toContain("Hoje");
    await fill(user);
    await user.click(row("Quando"));
    await user.click(screen.getByRole("button", { name: "Ontem" }));
    expect(row("Quando").textContent).toContain("Ontem");
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(bodyOf(fetch).date).toBe(yesterday());

    await user.click(row("Quando"));
    fireEvent.change(screen.getByLabelText("Data do lançamento"), { target: { value: "2026-01-15" } });
    await user.click(screen.getByRole("button", { name: "Voltar" }));
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    expect(bodyOf(fetch, 1).date).toBe("2026-01-15");
  });

  it("Pago com: picks another payment method", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open();
    expect(row("Pago com").textContent).toContain("Pix");
    await fill(user);
    await user.click(row("Pago com"));
    await user.click(screen.getByRole("button", { name: "Crédito" }));
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch).paymentMethodId).toBe(2);
  });

  it("lists all 25 categories when there are that many", async () => {
    const many: Category[] = Array.from({ length: 25 }, (_, i) => ({ ...categories[0], id: 100 + i, name: `Categoria ${i}`, sortOrder: i }));
    const { user } = open({}, appCtx({ categories: many }));
    await user.click(row("Categoria"));
    expect(document.querySelectorAll(".cat-tile")).toHaveLength(25);
  });
});

describe("AddSheet: splitting", () => {
  const splitting = () => appCtx({ wallet: { ...wallet, splittingEnabled: true } });

  it("shows no splitting on a wallet without splitting", () => {
    open();
    expect(screen.queryByRole("button", { name: "Ajustar" })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Quem pagou/ })).toBeNull();
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
    expect(row("Quem pagou").textContent).toContain("Ana");
    await user.click(row("Quem pagou"));
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

  it("keeps the transaction's archived category selectable, and offers no other archived one", async () => {
    const archived = (id: number, name: string): Category => ({ ...categories[0], id, name, archived: true });
    const ctx = appCtx({ categories: [...categories, archived(8, "Antiga"), archived(9, "Outra antiga")] });
    const { user } = open({ tx: tx({ categoryId: 8 }) }, ctx);
    expect(row("Categoria").textContent).toContain("Antiga");
    await user.click(row("Categoria"));
    expect(screen.getByRole("button", { name: /^Antiga$/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Outra antiga/ })).toBeNull();
  });
});
