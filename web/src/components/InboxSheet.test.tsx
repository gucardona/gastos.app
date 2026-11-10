import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { InboxItem } from "../api/types";
import { appCtx, bodyOf, mockFetch, renderApp } from "../test/fixtures";
import { InboxSheet } from "./InboxSheet";

const line = (over: Partial<InboxItem> = {}): InboxItem => ({
  id: 5, bankAccountId: 1, accountName: "Nubank", date: "2026-10-01", amountCents: -4890, description: "COMPRA MERCADO DO BAIRRO",
  status: "pending", suggestedCategoryId: null, suggestedDescription: null, suggestedMatch: null, ...over,
});

const serve = () => mockFetch({ body: {} });
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n: number) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const row = (label: string) => screen.getByRole("button", { name: new RegExp(`^${label}`) });

function open(item: InboxItem | null = line(), ctx = appCtx()) {
  const onClose = vi.fn();
  const onDone = vi.fn();
  renderApp(<InboxSheet item={item} onDone={onDone} onClose={onClose} />, ctx);
  return { onClose, onDone, user: userEvent.setup() };
}
async function pickCategory(user: ReturnType<typeof userEvent.setup>, name: RegExp) {
  await user.click(row("Categoria"));
  await user.click(screen.getByRole("button", { name }));
}

describe("InboxSheet", () => {
  it("is closed without a line", () => {
    open(null);
    expect(screen.queryByRole("button", { name: "Aceitar" })).toBeNull();
  });

  it("shows what the line is: amount and account", () => {
    open();
    expect(screen.getByText(/48,90/)).toBeTruthy();
    expect(screen.getByText(/Nubank/)).toBeTruthy();
  });

  it("cannot accept before a category is chosen, and says why next to the button", async () => {
    const { user } = open();
    const accept = screen.getByRole("button", { name: "Aceitar" }) as HTMLButtonElement;
    expect(accept.disabled).toBe(true);
    expect(document.getElementById(accept.getAttribute("aria-describedby")!)?.textContent).toBe("Escolha uma categoria para aceitar");
    await pickCategory(user, /Mercado/);
    expect(screen.queryByText("Escolha uma categoria para aceitar")).toBeNull();
  });

  it("accepts with the chosen category and reports it done", async () => {
    const fetch = serve();
    const { user, onDone, onClose } = open();
    await pickCategory(user, /Mercado/);
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    await waitFor(() => expect(onDone).toHaveBeenCalledWith("Lançamento criado"));
    expect(onClose).not.toHaveBeenCalled(); // the page decides: the next line, or closing
    expect(writes(fetch)).toHaveLength(1);
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/inbox/5/accept");
    expect(bodyAt(fetch, 0)).toEqual({ categoryId: 1 });
  });

  it("only offers the categories of the kind of the line", async () => {
    const { user } = open(line({ amountCents: 850000, description: "SALARIO" }));
    await user.click(row("Categoria"));
    expect(screen.getByRole("button", { name: /Salário/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Mercado/ })).toBeNull();
  });

  it("starts from the suggested category and name, and offers no rule when the category is already suggested", () => {
    open(line({ suggestedCategoryId: 1, suggestedDescription: "Mercado" }));
    expect(row("Categoria").textContent).toContain("Mercado");
    expect((screen.getByLabelText("Descrição") as HTMLInputElement).value).toBe("Mercado");
    expect(screen.queryByRole("switch", { name: "Usar nas próximas importações" })).toBeNull();
  });

  it("an edited description is sent with the category", async () => {
    const fetch = serve();
    const { user } = open();
    await pickCategory(user, /Mercado/);
    await user.clear(screen.getByLabelText("Descrição"));
    await user.type(screen.getByLabelText("Descrição"), "Mercado do Zé");
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch, 0)).toEqual({ categoryId: 1, description: "Mercado do Zé" });
  });

  it("creates a rule after accepting when asked, starting from the first two words", async () => {
    const fetch = serve();
    const { user } = open();
    await pickCategory(user, /Mercado/);
    await user.click(screen.getByRole("switch", { name: "Usar nas próximas importações" }));
    expect((screen.getByLabelText("Texto da regra") as HTMLInputElement).value).toBe("compra mercado");
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(2));
    expect(writes(fetch)[1][0]).toBe("/api/wallets/1/rules");
    expect(bodyAt(fetch, 1)).toEqual({ pattern: "compra mercado", categoryId: 1 });
  });

  it("a rule text under 2 characters is refused before the line is accepted", async () => {
    const fetch = serve();
    const { user, onDone } = open();
    await pickCategory(user, /Mercado/);
    await user.click(screen.getByRole("switch", { name: "Usar nas próximas importações" }));
    fireEvent.change(screen.getByLabelText("Texto da regra"), { target: { value: "a" } });
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    expect(await screen.findByText("Use ao menos 2 caracteres na regra.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
    expect(onDone).not.toHaveBeenCalled();
  });

  it("links to the look-alike transaction instead of creating one", async () => {
    const fetch = serve();
    const { user, onDone } = open(line({ suggestedMatch: { id: 77, description: "Mercado (digitado)", date: "2026-10-02" } }));
    expect(screen.getByText(/Parece o lançamento que você já fez: Mercado \(digitado\)/)).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "É este lançamento" }));
    await waitFor(() => expect(onDone).toHaveBeenCalledWith("Ligado ao lançamento"));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/inbox/5/match");
    expect(bodyAt(fetch, 0)).toEqual({ transactionId: 77 });
  });

  it("offers no link when there is no look-alike", () => {
    open();
    expect(screen.queryByRole("button", { name: "É este lançamento" })).toBeNull();
  });

  it("ignores the line and reports it done", async () => {
    const fetch = serve();
    const { user, onDone } = open();
    await user.click(screen.getByRole("button", { name: "Ignorar" }));
    await waitFor(() => expect(onDone).toHaveBeenCalledWith("Linha ignorada"));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/inbox/5/ignore");
  });

  it("a line someone else already handled shows the message of the server and stays open", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 409, body: { error: { code: "conflict", message: "Esta linha já foi tratada" } } } : { body: [] }));
    const { user, onDone } = open();
    await user.click(screen.getByRole("button", { name: "Ignorar" }));
    expect(await screen.findByText("Esta linha já foi tratada")).toBeTruthy();
    expect(onDone).not.toHaveBeenCalled();
  });
});

describe("InboxSheet: the rule step", () => {
  const rulesFail = () =>
    mockFetch((url, init) => {
      if (init.method === "POST" && url.endsWith("/rules")) return { status: 409, body: { error: { code: "conflict", message: "Já existe uma regra com esse texto" } } };
      return { body: {} };
    });

  it("when the rule is refused after the line was accepted, it says so and offers only to close", async () => {
    rulesFail();
    const { user, onClose } = open();
    await pickCategory(user, /Mercado/);
    await user.click(screen.getByRole("switch", { name: "Usar nas próximas importações" }));
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    expect(await screen.findByText("Lançamento criado, mas a regra não foi salva.")).toBeTruthy();
    expect(screen.getByText("Já existe uma regra com esse texto")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Aceitar" })).toBeNull(); // accepting again would only give a 409
    expect(onClose).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Fechar" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("a rule text over 60 characters is refused before the line is accepted", async () => {
    const fetch = serve();
    const { user } = open();
    await pickCategory(user, /Mercado/);
    await user.click(screen.getByRole("switch", { name: "Usar nas próximas importações" }));
    fireEvent.change(screen.getByLabelText("Texto da regra"), { target: { value: "x".repeat(61) } });
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    expect(await screen.findByText("Use no máximo 60 caracteres na regra.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("the rule message goes away as soon as the text is edited", async () => {
    serve();
    const { user } = open();
    await pickCategory(user, /Mercado/);
    await user.click(screen.getByRole("switch", { name: "Usar nas próximas importações" }));
    fireEvent.change(screen.getByLabelText("Texto da regra"), { target: { value: "a" } });
    await user.click(screen.getByRole("button", { name: "Aceitar" }));
    expect(await screen.findByText("Use ao menos 2 caracteres na regra.")).toBeTruthy();
    await user.type(screen.getByLabelText("Texto da regra"), "bc");
    expect(screen.queryByText("Use ao menos 2 caracteres na regra.")).toBeNull();
  });

  it("the notice of what was done before this line is a status, announced as the line changes", () => {
    renderApp(<InboxSheet item={line({ id: 6, description: "TARIFA BANCARIA" })} notice="Lançamento criado. 2 restantes." onDone={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText("Lançamento criado. 2 restantes.").getAttribute("role")).toBe("status");
  });
});
