import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { BankAccount } from "../api/types";
import { bodyOf, mockFetch, renderApp } from "../test/fixtures";
import { BankAccountSheet } from "./BankAccountSheet";

const account: BankAccount = { id: 3, name: "Nubank", paymentMethodId: null, paidByUserId: 1, enabled: true, pendingCount: 2 };
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const serve = (reply: unknown = {}, status = 200) => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { status, body: reply } : { body: [] }));

function open(a: BankAccount | "new" | null) {
  const onClose = vi.fn();
  renderApp(<BankAccountSheet account={a} onClose={onClose} />);
  return { onClose, user: userEvent.setup() };
}
const file = (name = "extrato.ofx", content = "<OFX></OFX>") => new File([content], name);

describe("BankAccountSheet: new", () => {
  it("is closed without an account", () => {
    open(null);
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("creates an account with a name and closes", async () => {
    const fetch = serve({}, 201);
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Nome da conta bancária"), "Itaú");
    await user.click(screen.getByRole("button", { name: "Criar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/bank-accounts");
    expect(bodyAt(fetch)).toEqual({ name: "Itaú" });
  });

  it("shows the error of the API and stays open", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { name: "obrigatório, até 60 caracteres" } } } } : { body: [] }));
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Nome da conta bancária"), "x");
    await user.click(screen.getByRole("button", { name: "Criar" }));
    expect(await screen.findByText("obrigatório, até 60 caracteres")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("BankAccountSheet: settings of an account", () => {
  it("changes the payment method, keeping the other fields", async () => {
    const fetch = serve();
    const { user } = open(account);
    expect(screen.getByRole("button", { name: /^Forma de pagamento/ }).textContent).toContain("Nenhuma");
    await user.click(screen.getByRole("button", { name: /^Forma de pagamento/ }));
    await user.click(screen.getByRole("button", { name: "Crédito" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/bank-accounts/3");
    expect(writes(fetch)[0][1]!.method).toBe("PUT");
    expect(bodyAt(fetch)).toEqual({ name: "Nubank", paymentMethodId: 2, enabled: true });
  });

  it("can go back to no payment method", async () => {
    const fetch = serve();
    const { user } = open({ ...account, paymentMethodId: 2 });
    await user.click(screen.getByRole("button", { name: /^Forma de pagamento/ }));
    await user.click(screen.getByRole("button", { name: "Nenhuma" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch).paymentMethodId).toBeNull();
  });

  it("turns the account off with a switch, and a disabled account offers no import", async () => {
    const fetch = serve();
    const { user } = open(account);
    await user.click(screen.getByRole("switch", { name: "Conta ativa" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toEqual({ name: "Nubank", paymentMethodId: null, enabled: false });
  });

  it("a disabled account shows no import form", () => {
    open({ ...account, enabled: false });
    expect(screen.queryByLabelText("Arquivo do extrato")).toBeNull();
    expect(screen.getByRole("switch", { name: "Conta ativa" }).getAttribute("aria-checked")).toBe("false");
  });

  it("deletes only after confirming", async () => {
    const fetch = serve(undefined, 204);
    const { user, onClose } = open(account);
    await user.click(screen.getByRole("button", { name: "Excluir conta" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/bank-accounts/3");
  });
});

describe("BankAccountSheet: importing a statement", () => {
  it("uploads an OFX statement and reports what was new", async () => {
    const fetch = serve({ imported: 3, duplicates: 1 });
    const { user } = open(account);
    await user.upload(screen.getByLabelText("Arquivo do extrato"), file());
    await user.click(screen.getByRole("button", { name: "Importar" }));
    expect(await screen.findByText("3 linhas novas, 1 já importada antes.")).toBeTruthy();
    const [url, init] = writes(fetch)[0];
    expect(url).toBe("/api/wallets/1/bank-accounts/3/import");
    const form = init!.body as FormData;
    expect(form.get("format")).toBe("ofx");
    expect((form.get("file") as File).name).toBe("extrato.ofx");
    expect(init!.headers).toBeUndefined(); // the browser sets the multipart boundary
  });

  it("asks for the CSV columns only for CSV, and sends them", async () => {
    const fetch = serve({ imported: 1, duplicates: 0 });
    const { user } = open(account);
    expect(screen.queryByLabelText("Coluna da data")).toBeNull();
    await user.click(screen.getByRole("button", { name: "CSV" }));
    await user.upload(screen.getByLabelText("Arquivo do extrato"), file("fatura.csv", "a;b;c"));
    await user.click(screen.getByLabelText(/Compras aparecem como valores positivos/));
    await user.click(screen.getByRole("button", { name: "Importar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    const form = writes(fetch)[0][1]!.body as FormData;
    expect([form.get("format"), form.get("dateColumn"), form.get("descriptionColumn"), form.get("amountColumn"), form.get("hasHeader"), form.get("invertSign")])
      .toEqual(["csv", "1", "2", "3", "true", "true"]);
  });

  it("cannot import without choosing a file", () => {
    open(account);
    expect((screen.getByRole("button", { name: "Importar" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("shows why a file was refused", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 400, body: { error: { code: "validation", message: "Este arquivo não parece ser um OFX." } } } : { body: [] }));
    const { user } = open(account);
    await user.upload(screen.getByLabelText("Arquivo do extrato"), file("x.ofx", "x"));
    await user.click(screen.getByRole("button", { name: "Importar" }));
    expect(await screen.findByText("Este arquivo não parece ser um OFX.")).toBeTruthy();
  });
});
