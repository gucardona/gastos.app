import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { BankAccount } from "../../api/types";
import { bodyOf, mockFetch, renderApp } from "../../test/fixtures";
import { BankSection } from "./BankSection";

const account: BankAccount = { id: 3, name: "Nubank", paymentMethodId: null, paidByUserId: 1, enabled: true, pendingCount: 2 };

const serve = (reply: Record<string, unknown> = {}) =>
  mockFetch((url, init) => {
    if (init.method && init.method !== "GET") return { body: reply };
    return { body: url.endsWith("/bank-accounts") ? [account] : [] };
  });
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");

describe("Settings: bank accounts", () => {
  it("lists the accounts with what is waiting for review", async () => {
    serve();
    renderApp(<BankSection />);
    expect(await screen.findByText("Nubank")).toBeTruthy();
    expect(screen.getByText("2 para revisar")).toBeTruthy();
  });

  it("creates an account", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.click(await screen.findByRole("button", { name: "Nova conta bancária" }));
    await user.type(screen.getByLabelText("Nome da conta bancária"), "Itaú");
    await user.click(screen.getByRole("button", { name: "Criar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/bank-accounts");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ name: "Itaú" });
  });

  it("uploads an OFX statement and reports what was new", async () => {
    const fetch = serve({ imported: 3, duplicates: 1 });
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.click(await screen.findByRole("button", { name: "Importar extrato" }));
    await user.upload(screen.getByLabelText("Arquivo do extrato"), new File(["<OFX></OFX>"], "extrato.ofx"));
    await user.click(screen.getByRole("button", { name: "Importar" }));
    expect(await screen.findByText("3 linhas novas, 1 já importada antes.")).toBeTruthy();
    const [url, init] = writes(fetch)[0];
    expect(url).toBe("/api/wallets/1/bank-accounts/3/import");
    const form = init!.body as FormData;
    expect(form.get("format")).toBe("ofx");
    expect((form.get("file") as File).name).toBe("extrato.ofx");
    expect(init!.headers).toBeUndefined(); // the browser sets the multipart boundary
  });

  it("asks for the CSV columns and sends them", async () => {
    const fetch = serve({ imported: 1, duplicates: 0 });
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.click(await screen.findByRole("button", { name: "Importar extrato" }));
    await user.selectOptions(screen.getByLabelText("Formato do extrato"), "csv");
    await user.upload(screen.getByLabelText("Arquivo do extrato"), new File(["a;b;c"], "fatura.csv"));
    await user.click(screen.getByLabelText(/Compras aparecem como valores positivos/));
    await user.click(screen.getByRole("button", { name: "Importar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    const form = writes(fetch)[0][1]!.body as FormData;
    expect([form.get("format"), form.get("dateColumn"), form.get("descriptionColumn"), form.get("amountColumn"), form.get("hasHeader"), form.get("invertSign")])
      .toEqual(["csv", "1", "2", "3", "true", "true"]);
  });

  it("cannot import without choosing a file", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.click(await screen.findByRole("button", { name: "Importar extrato" }));
    expect((screen.getByRole("button", { name: "Importar" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("shows why a file was refused", async () => {
    mockFetch((url, init) => init.method === "POST"
      ? { status: 400, body: { error: { code: "validation", message: "Este arquivo não parece ser um OFX." } } }
      : { body: url.endsWith("/bank-accounts") ? [account] : [] });
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.click(await screen.findByRole("button", { name: "Importar extrato" }));
    await user.upload(screen.getByLabelText("Arquivo do extrato"), new File(["x"], "x.ofx"));
    await user.click(screen.getByRole("button", { name: "Importar" }));
    expect(await screen.findByText("Este arquivo não parece ser um OFX.")).toBeTruthy();
  });

  it("keeps the other fields when the payment method changes", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<BankSection />);
    await user.selectOptions(await screen.findByLabelText("Forma de pagamento de Nubank"), "2");
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toEqual({ name: "Nubank", paymentMethodId: 2, enabled: true });
  });

  it("deletes a rule", async () => {
    const fetch = mockFetch((url, init) => {
      if (init.method === "DELETE") return { status: 204 };
      return { body: url.endsWith("/rules") ? [{ id: 8, pattern: "uber", categoryId: 1, renameTo: "Uber", priority: 0 }] : [] };
    });
    const user = userEvent.setup();
    renderApp(<BankSection />);
    expect(await screen.findByText(/renomeia para Uber/)).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Excluir regra uber" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/rules/8");
  });
});
