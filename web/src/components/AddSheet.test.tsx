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

describe("AddSheet: novo lançamento", () => {
  it("não envia e mostra o que falta quando o formulário está vazio", async () => {
    const fetch = mockFetch();
    const { user } = open();
    await user.click(save());
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(screen.getByText("Descreva o lançamento.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("cria o gasto com categoria e pagamento padrão e fecha", async () => {
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

  it("troca para entrada e usa as categorias de entrada", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open();
    await user.click(screen.getByRole("button", { name: "Entrada" }));
    expect(screen.queryByRole("button", { name: /Mercado/ })).toBeNull();
    await fill(user, "8500", "Salário de outubro");
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch)).toMatchObject({ kind: "income", amountCents: 850000, categoryId: 3 });
  });

  it("'Salvar e lançar outro' mantém o sheet aberto e limpa valor e descrição", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user, onClose } = open();
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Salvar e lançar outro" }));
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    await waitFor(() => expect((screen.getByRole("textbox", { name: "Valor" }) as HTMLInputElement).value).toBe(""));
    expect((screen.getByRole("textbox", { name: "Descrição" }) as HTMLInputElement).value).toBe("");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("mostra o erro de campo devolvido pela API", async () => {
    mockFetch({ status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { categoryId: "categoria arquivada" } } } });
    const { user, onClose } = open();
    await fill(user);
    await user.click(save());
    expect(await screen.findByText("categoria arquivada")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("mostra a mensagem geral quando a API falha sem campos", async () => {
    mockFetch({ status: 403, body: { error: { code: "forbidden", message: "Sem permissão para esta ação" } } });
    const { user } = open();
    await fill(user);
    await user.click(save());
    expect(await screen.findByText("Sem permissão para esta ação")).toBeTruthy();
  });

  it("'Repetir todo mês' cria uma recorrência a partir da data", async () => {
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

describe("AddSheet: divisão", () => {
  const splitting = () => appCtx({ wallet: { ...wallet, splittingEnabled: true } });

  it("não mostra divisão numa carteira sem divisão", () => {
    open();
    expect(screen.queryByRole("button", { name: "Ajustar" })).toBeNull();
  });

  it("recusa porcentagens que não somam 100% e envia quando fecham", async () => {
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

  it("divisão padrão não envia splits (o servidor divide por igual)", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open({}, splitting());
    await fill(user);
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch)).not.toHaveProperty("splits");
  });

  it("permite escolher quem pagou", async () => {
    const fetch = mockFetch({ status: 201, body: tx() });
    const { user } = open({}, splitting());
    await fill(user);
    await user.click(screen.getByRole("button", { name: "Bia" }));
    await user.click(save());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(bodyOf(fetch).paidByUserId).toBe(2);
  });
});

describe("AddSheet: edição", () => {
  it("preenche com o lançamento e salva com PUT", async () => {
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

  it("exclui só depois de confirmar", async () => {
    const fetch = mockFetch({ status: 204 });
    const { user, onClose } = open({ tx: tx() });
    await user.click(screen.getByRole("button", { name: "Excluir lançamento" }));
    expect(fetch).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(fetch.mock.calls[0][1]!.method).toBe("DELETE");
    expect(fetch.mock.calls[0][0]).toBe("/api/wallets/1/transactions/10");
  });

  it("não oferece 'Repetir todo mês' nem 'lançar outro' ao editar", () => {
    open({ tx: tx() });
    expect(screen.queryByRole("switch", { name: "Repetir todo mês" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Salvar e lançar outro" })).toBeNull();
  });
});
