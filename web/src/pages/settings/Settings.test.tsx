import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Category } from "../../api/types";
import { appCtx, bodyOf, categories, mockFetch, renderApp, wallet } from "../../test/fixtures";
import { BudgetsSection } from "./BudgetsSection";
import { CategoriesSection } from "./CategoriesSection";
import { MembersSection } from "./MembersSection";
import { PaymentsSection } from "./PaymentsSection";
import { WalletSection } from "./WalletSection";

const writes = (fetch: ReturnType<typeof mockFetch>) => fetch.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const lastWrite = (fetch: ReturnType<typeof mockFetch>) => {
  const w = writes(fetch);
  return { url: w[w.length - 1][0] as string, method: w[w.length - 1][1]!.method, body: w[w.length - 1][1]!.body ? bodyOf(fetch, fetch.mock.calls.indexOf(w[w.length - 1])) : undefined };
};
const ok = (extra: Record<string, unknown> = {}) => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: extra } : { body: [] }));

describe("Ajustes: membros", () => {
  it("convida por e-mail com a permissão escolhida", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.type(screen.getByLabelText("E-mail do convidado"), "caio@example.com");
    await user.selectOptions(screen.getByLabelText("Permissão do convidado"), "reader");
    await user.click(screen.getByRole("button", { name: "Convidar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/members", method: "POST", body: { email: "caio@example.com", role: "reader" } });
  });

  it("mostra o erro quando o e-mail não tem conta", async () => {
    mockFetch({ status: 404, body: { error: { code: "not_found", message: "Usuário não encontrado" } } });
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    await user.type(screen.getByLabelText("E-mail do convidado"), "x@example.com");
    await user.click(screen.getByRole("button", { name: "Convidar" }));
    expect(await screen.findByText("Usuário não encontrado")).toBeTruthy();
  });

  it("a dona muda a permissão de outro membro, mas não a própria", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<MembersSection />);
    expect(screen.queryByLabelText("Permissão de Ana Souza")).toBeNull();
    await user.selectOptions(screen.getByLabelText("Permissão de Bia Lima"), "reader");
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/members/2", method: "PATCH", body: { role: "reader" } });
  });

  it("quem não é dona não convida nem remove ninguém, mas pode sair", () => {
    const bia = { ...wallet, role: "editor" as const };
    renderApp(<MembersSection />, appCtx({ user: { id: 2, name: "Bia Lima", email: "bia@example.com" }, wallet: bia }));
    expect(screen.queryByRole("button", { name: "Convidar" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Remover" })).toBeNull();
    expect(screen.getByRole("button", { name: "Sair da carteira" })).toBeTruthy();
  });
});

describe("Ajustes: categorias", () => {
  it("cria uma categoria de gasto com tipo de gasto", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<CategoriesSection />);
    await user.click(screen.getByRole("button", { name: "Nova categoria" }));
    await user.type(screen.getByLabelText("Nome da categoria"), "Pets");
    await user.type(screen.getByLabelText("Ícone da categoria"), "🐶");
    await user.selectOptions(screen.getByLabelText("Tipo de gasto"), "nonessential");
    await user.click(screen.getByRole("button", { name: "Criar categoria" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/categories", method: "POST", body: { kind: "expense", name: "Pets", icon: "🐶", essentiality: "nonessential" } });
  });

  it("entradas não têm tipo de gasto", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<CategoriesSection />);
    await user.click(screen.getByRole("button", { name: "Entradas" }));
    expect(screen.getByText("Salário")).toBeTruthy();
    expect(screen.queryByText("Mercado")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Nova categoria" }));
    expect(screen.queryByLabelText("Tipo de gasto")).toBeNull();
    await user.type(screen.getByLabelText("Nome da categoria"), "Bônus");
    await user.type(screen.getByLabelText("Ícone da categoria"), "🎁");
    await user.click(screen.getByRole("button", { name: "Criar categoria" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch).body).not.toHaveProperty("essentiality");
  });

  it("mostra o erro da API, como nome repetido", async () => {
    mockFetch({ status: 409, body: { error: { code: "conflict", message: "Já existe uma categoria com esse nome" } } });
    const user = userEvent.setup();
    renderApp(<CategoriesSection />);
    await user.click(screen.getByRole("button", { name: "Nova categoria" }));
    await user.type(screen.getByLabelText("Nome da categoria"), "Mercado");
    await user.type(screen.getByLabelText("Ícone da categoria"), "🛒");
    await user.click(screen.getByRole("button", { name: "Criar categoria" }));
    expect(await screen.findByText("Já existe uma categoria com esse nome")).toBeTruthy();
  });

  it("explica quando a exclusão virou arquivamento", async () => {
    const archived: Category = { ...categories[0], archived: true };
    mockFetch({ status: 200, body: archived });
    const user = userEvent.setup();
    renderApp(<CategoriesSection />);
    const row = screen.getByText("Mercado").closest(".item") as HTMLElement;
    await user.click(row.querySelectorAll("button")[1]); // Excluir
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    expect(await screen.findByText("Arquivada, porque já tem lançamentos.")).toBeTruthy();
  });

  it("restaura uma categoria arquivada", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    const old = { ...categories[1], archived: true };
    renderApp(<CategoriesSection />, appCtx({ categories: [categories[0], old, categories[2]] }));
    await user.click(screen.getByRole("button", { name: /Mostrar arquivadas \(1\)/ }));
    await user.click(screen.getByRole("button", { name: "Restaurar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/categories/2", method: "PUT", body: { archived: false, name: "Restaurante" } });
  });
});

describe("Ajustes: formas de pagamento", () => {
  it("cria uma forma de pagamento", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<PaymentsSection />);
    await user.click(screen.getByRole("button", { name: "Nova forma de pagamento" }));
    await user.type(screen.getByLabelText("Nome da forma de pagamento"), "Cheque");
    await user.type(screen.getByLabelText("Ícone da forma de pagamento"), "🧾");
    await user.click(screen.getByRole("button", { name: "Criar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/payment-methods", method: "POST", body: { name: "Cheque", icon: "🧾" } });
  });
});

describe("Ajustes: orçamentos", () => {
  const withBudget = () => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: [{ categoryId: 1, limitCents: 150000 }] }));

  it("mostra o limite atual e salva um novo em centavos", async () => {
    const fetch = withBudget();
    const user = userEvent.setup();
    renderApp(<BudgetsSection />);
    const input = (await screen.findByLabelText("Limite mensal de Mercado")) as HTMLInputElement;
    expect(input.value).toBe("1500,00");
    await user.clear(input);
    await user.type(input, "1.800,50");
    await user.click(input.closest("form")!.querySelector("button")!);
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/budgets/1", method: "PUT", body: { limitCents: 180050 } });
  });

  it("recusa valor inválido sem chamar a API", async () => {
    const fetch = withBudget();
    const user = userEvent.setup();
    renderApp(<BudgetsSection />);
    const input = await screen.findByLabelText("Limite mensal de Restaurante");
    await user.type(input, "abc");
    await user.click(input.closest("form")!.querySelector("button")!);
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("remove o limite", async () => {
    const fetch = withBudget();
    const user = userEvent.setup();
    renderApp(<BudgetsSection />);
    await screen.findByLabelText("Limite mensal de Mercado");
    await user.click(screen.getByRole("button", { name: "Remover" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1/budgets/1", method: "DELETE" });
  });

  it("só lista categorias de gasto ativas", async () => {
    withBudget();
    renderApp(<BudgetsSection />);
    await screen.findByLabelText("Limite mensal de Mercado");
    expect(screen.queryByLabelText("Limite mensal de Salário")).toBeNull();
  });
});

describe("Ajustes: carteira", () => {
  it("liga a divisão de gastos", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("switch", { name: "Dividir gastos entre os membros" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1", method: "PATCH", body: { splittingEnabled: true } });
  });

  it("cria uma nova carteira e passa a usá-la", async () => {
    const created = { ...wallet, id: 9, name: "Viagem" };
    const ctx = appCtx();
    const fetch = mockFetch((_u, init) => (init.method === "POST" ? { status: 201, body: created } : { body: [] }));
    const user = userEvent.setup();
    renderApp(<WalletSection />, ctx);
    await user.type(screen.getByLabelText("Nome da nova carteira"), "Viagem");
    await user.click(screen.getByRole("button", { name: "Criar carteira" }));
    await waitFor(() => expect(ctx.setWalletId).toHaveBeenCalledWith(9));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets", method: "POST", body: { name: "Viagem", splittingEnabled: false } });
  });

  it("exclui a carteira só depois de confirmar", async () => {
    const fetch = ok();
    const user = userEvent.setup();
    renderApp(<WalletSection />);
    await user.click(screen.getByRole("button", { name: "Excluir" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(lastWrite(fetch)).toMatchObject({ url: "/api/wallets/1", method: "DELETE" });
  });

  it("quem não é dona não edita a carteira", () => {
    renderApp(<WalletSection />, appCtx({ wallet: { ...wallet, role: "editor" } }));
    expect(screen.queryByRole("button", { name: "Excluir" })).toBeNull();
    expect((screen.getByLabelText("Nome da carteira") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByRole("switch", { name: "Dividir gastos entre os membros" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
