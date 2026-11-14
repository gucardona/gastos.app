import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Category } from "../api/types";
import { today, yesterday } from "../lib/dates";
import { appCtx, bodyOf, categories, members, mockFetch, renderApp, tx, wallet } from "../test/fixtures";
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

describe("AddSheet: the amount fills from the cents", () => {
  it("typing 587 shows 5,87 and saves 587 cents; typing 58700 saves R$ 587,00", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open();
    const amount = screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement;
    expect(amount.inputMode).toBe("numeric");
    await user.type(amount, "587");
    expect(amount.value).toBe("5,87");
    await user.clear(amount);
    await user.type(amount, "58700");
    expect(amount.value).toBe("587,00");
    await pickCategory(user, /Restaurante/);
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch)).toMatchObject({ amountCents: 58700 });
  });

  it("backspace takes the last digit off", async () => {
    mockFetch();
    const { user } = open();
    const amount = screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement;
    await user.type(amount, "587{Backspace}");
    expect(amount.value).toBe("0,58");
  });
});

describe("AddSheet: a recurrence can end", () => {
  async function repeating(user: ReturnType<typeof userEvent.setup>) {
    await user.type(screen.getByRole("textbox", { name: "Valor" }), "8990");
    await pickCategory(user, /Restaurante/);
    await user.click(screen.getByRole("switch", { name: "Repetir todo mês" }));
  }

  it("with Repetir todo mês on, an optional Data final appears and is sent with the recurrence", async () => {
    const fetch = mockFetch({ status: 201, body: {} });
    const { user } = open();
    expect(screen.queryByLabelText("Data final (opcional)")).toBeNull();
    await repeating(user);
    const end = screen.getByLabelText("Data final (opcional)") as HTMLInputElement;
    expect(end.type).toBe("date");
    fireEvent.change(end, { target: { value: "2099-12-31" } });
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/recurrences");
    expect(bodyOf(fetch)).toMatchObject({ amountCents: 8990, endDate: "2099-12-31" });
  });

  it("without a Data final the recurrence has no end, as before", async () => {
    const fetch = mockFetch({ status: 201, body: {} });
    const { user } = open();
    await repeating(user);
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch).endDate).toBeUndefined();
  });

  it("a Data final before the start is refused before anything is sent", async () => {
    const fetch = mockFetch({ status: 201, body: {} });
    const { user } = open();
    await repeating(user);
    fireEvent.change(screen.getByLabelText("Data final (opcional)"), { target: { value: "2000-01-01" } });
    await user.click(save());
    expect(await screen.findByText("A data final precisa ser no dia do início ou depois.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
  });
});

describe("AddSheet: new transaction", () => {
  it("does not submit and shows what is missing when the form is empty", async () => {
    const fetch = mockFetch();
    const { user } = open();
    await user.click(save());
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(screen.queryByText("Descreva o lançamento.")).toBeNull(); // the description is optional
    expect(fetch).not.toHaveBeenCalled();
  });

  it("an empty amount points the field at its error (typing can never give more than 2 decimals)", async () => {
    const fetch = mockFetch();
    const { user } = open();
    const amount = screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement;
    expect(amount.getAttribute("aria-invalid")).toBeNull();
    await user.type(amount, "12,345");
    expect(amount.value).toBe("123,45");
    await user.clear(amount);
    await user.click(save());
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(amount.getAttribute("aria-invalid")).toBe("true");
    expect(document.getElementById(amount.getAttribute("aria-describedby")!)?.textContent).toBe("Informe um valor maior que zero.");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("a description the API refuses points the field at its error", async () => {
    mockFetch({ status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { description: "use até 200 caracteres" } } } });
    const { user } = open();
    await fill(user);
    await user.click(save());
    await screen.findByText("use até 200 caracteres");
    const desc = screen.getByRole("textbox", { name: "Descrição" });
    expect(desc.getAttribute("aria-invalid")).toBe("true");
    expect(document.getElementById(desc.getAttribute("aria-describedby")!)?.textContent).toBe("use até 200 caracteres");
  });

  it("where CSS field-sizing is missing, the amount field still grows with what is typed", async () => {
    mockFetch();
    vi.stubGlobal("CSS", { supports: () => false });
    const { user } = open();
    const amount = screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement;
    await user.type(amount, "1.234.567,89");
    expect(parseFloat(amount.style.width)).toBeGreaterThanOrEqual("1.234.567,89".length);
  });

  it("where CSS field-sizing works, it alone sizes the amount field", () => {
    mockFetch();
    vi.stubGlobal("CSS", { supports: () => true });
    open();
    expect((screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement).style.width).toBe("");
  });

  it("saves without a description", async () => {
    const fetch = mockFetch({ status: 201, body: tx({ description: "" }) });
    const { user, onClose } = open();
    await user.type(screen.getByRole("textbox", { name: "Valor" }), "48,90");
    await pickCategory(user, /Restaurante/);
    await user.click(save());
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(bodyOf(fetch)).toMatchObject({ amountCents: 4890, description: "" });
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
    await fill(user, "850000", "Salário de outubro");
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

  it("a field error with no place of its own on the sheet shows in the general error area", async () => {
    mockFetch({ status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { paymentMethodId: "forma de pagamento não encontrada" } } } });
    const { user, onClose } = open();
    await fill(user);
    await user.click(save());
    expect((await screen.findByRole("alert")).textContent).toBe("Forma de pagamento não encontrada.");
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
    ["1000", 1000],
    ["10", 10],
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

describe("AddSheet: focus", () => {
  it("opens with the amount field focused", () => {
    open();
    expect(document.activeElement).toBe(screen.getByRole("textbox", { name: "Valor" }));
  });

  it("opening a picker moves focus to its title, so the change is announced and Tab starts inside it", async () => {
    const { user } = open();
    await user.click(row("Categoria"));
    expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Categoria" }));
  });

  it("coming back from a picker puts focus on the row, not on the amount field", async () => {
    const { user } = open();
    await user.click(row("Pago com"));
    await user.click(screen.getByRole("button", { name: "Voltar" }));
    expect(document.activeElement).toBe(row("Pago com"));
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
    await fill(user, "1200", "Café");
    await user.click(row("Categoria"));
    await user.click(screen.getByRole("button", { name: "Voltar" }));
    expect((screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement).value).toBe("12,00");
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

  it("an income says Recebido por instead of Pago com, in the row and in its picker", async () => {
    mockFetch();
    const { user } = open();
    await user.click(screen.getByRole("button", { name: "Entrada" }));
    expect(screen.queryByText("Pago com")).toBeNull();
    await user.click(row("Recebido por"));
    expect(screen.getByRole("heading", { name: "Recebido por" })).toBeTruthy();
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

  const withShares = () => appCtx({ wallet: { ...wallet, splittingEnabled: true }, members: members.map((m) => ({ ...m, defaultShareBp: m.userId === 1 ? 6000 : 4000 })) });

  it("summarizes the default shares as percentages instead of an amount for each", async () => {
    const { user } = open({}, withShares());
    await fill(user);
    expect(screen.getByText("Ana 60%, Bia 40%")).toBeTruthy();
    expect(screen.queryByText(/para cada/)).toBeNull();
  });

  it("without shares the summary stays the equal amount for each", async () => {
    const { user } = open({}, splitting());
    await fill(user, "100,00");
    expect(screen.getByText(/R\$\s*50,00 para cada/)).toBeTruthy();
  });

  it("'Ajustar' starts from the default shares, and they are sent when kept", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open({}, withShares());
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Ajustar" }));
    expect((screen.getByRole("textbox", { name: "Porcentagem de Ana Souza" }) as HTMLInputElement).value).toBe("60");
    expect((screen.getByRole("textbox", { name: "Porcentagem de Bia Lima" }) as HTMLInputElement).value).toBe("40");
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch).splits).toEqual([{ userId: 1, shareBp: 6000 }, { userId: 2, shareBp: 4000 }]);
  });

  it("'Ajustar' without shares starts equal", async () => {
    const { user } = open({}, splitting());
    await user.click(screen.getByRole("button", { name: "Ajustar" }));
    expect((screen.getByRole("textbox", { name: "Porcentagem de Ana Souza" }) as HTMLInputElement).value).toBe("50");
  });

  it("'Repetir todo mês' sends the adjusted split with the recurrence", async () => {
    const fetch = mockFetch({ status: 201, body: {} });
    const { user } = open({}, splitting());
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Ajustar" }));
    await user.clear(screen.getByRole("textbox", { name: "Porcentagem de Ana Souza" }));
    await user.type(screen.getByRole("textbox", { name: "Porcentagem de Ana Souza" }), "70");
    await user.clear(screen.getByRole("textbox", { name: "Porcentagem de Bia Lima" }));
    await user.type(screen.getByRole("textbox", { name: "Porcentagem de Bia Lima" }), "30");
    await user.click(screen.getByRole("switch", { name: "Repetir todo mês" }));
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/recurrences");
    expect(bodyOf(fetch).splits).toEqual([{ userId: 1, shareBp: 7000 }, { userId: 2, shareBp: 3000 }]);
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

  it("a double click on Excluir does not confirm the deletion", async () => {
    const fetch = mockFetch({ status: 204 });
    const { user } = open({ tx: tx() });
    await user.dblClick(screen.getByRole("button", { name: "Excluir lançamento" }));
    expect(fetch).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Confirmar exclusão" })).toBeTruthy();
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

describe("AddSheet: toasts", () => {
  function openWithToast(state: { tx?: ReturnType<typeof tx> } = {}, ctx = appCtx()) {
    const onClose = vi.fn();
    const { toast } = renderApp(<AddSheet state={state} onClose={onClose} />, ctx);
    return { onClose, toast, user: userEvent.setup() };
  }

  it("says 'Lançamento salvo' after creating, also with 'Salvar e lançar outro'", async () => {
    mockFetch({ status: 201, body: tx() });
    const { user, toast } = openWithToast();
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Salvar e lançar outro" }));
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Lançamento salvo"));
  });

  it("says 'Lançamento salvo' after editing", async () => {
    mockFetch({ body: tx() });
    const { user, toast } = openWithToast({ tx: tx() });
    await user.click(save());
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Lançamento salvo"));
  });

  it("a monthly repeat says the recurrence was created", async () => {
    mockFetch({ status: 201, body: {} });
    const { user, toast } = openWithToast();
    await fill(user);
    await user.click(screen.getByRole("switch", { name: "Repetir todo mês" }));
    await user.click(save());
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Recorrência criada"));
  });

  it("after deleting, 'Desfazer' posts the same transaction again, splits included", async () => {
    const fetch = mockFetch({ status: 204 });
    const ctx = appCtx({ wallet: { ...wallet, splittingEnabled: true } });
    const deleted = tx({ splits: [{ userId: 1, shareBp: 6000, amountCents: 2934 }, { userId: 2, shareBp: 4000, amountCents: 1956 }] });
    const { user, toast } = openWithToast({ tx: deleted }, ctx);
    await user.click(screen.getByRole("button", { name: "Excluir lançamento" }));
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(toast).toHaveBeenCalled());
    const [message, opts] = toast.mock.calls[0];
    expect(message).toBe("Lançamento excluído");
    expect(opts.action.label).toBe("Desfazer");
    opts.action.run();
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    expect(fetch.mock.calls[1][0]).toBe("/api/wallets/1/transactions");
    expect(fetch.mock.calls[1][1]!.method).toBe("POST");
    expect(bodyOf(fetch, 1)).toEqual({
      kind: "expense", amountCents: 4890, description: "Mercado do bairro", categoryId: 1, paymentMethodId: 1, date: deleted.date,
      paidByUserId: 1, splits: [{ userId: 1, shareBp: 6000 }, { userId: 2, shareBp: 4000 }],
    });
  });

  it("a failed undo says why in a toast", async () => {
    let calls = 0;
    mockFetch(() => (calls++ === 0 ? { status: 204 } : { status: 400, body: { error: { code: "validation", message: "categoria arquivada" } } }));
    const { user, toast } = openWithToast({ tx: tx() });
    await user.click(screen.getByRole("button", { name: "Excluir lançamento" }));
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(toast).toHaveBeenCalled());
    toast.mock.calls[0][1].action.run();
    await waitFor(() => expect(toast).toHaveBeenLastCalledWith("categoria arquivada"));
  });

  it("a transaction made by a recurrence can be undone too: the server no longer makes a deleted occurrence again", async () => {
    mockFetch({ status: 204 });
    const { user, toast } = openWithToast({ tx: tx({ recurrenceId: 3, source: "recurring" }) });
    await user.click(screen.getByRole("button", { name: "Excluir lançamento" }));
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Lançamento excluído", expect.objectContaining({ action: expect.objectContaining({ label: "Desfazer" }) })));
  });
});
