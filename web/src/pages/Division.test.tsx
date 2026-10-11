import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Settlement, Summary } from "../api/types";
import { today } from "../lib/dates";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../test/fixtures";
import { Division } from "./Division";

const summary = (over: Partial<Summary> = {}): Summary => ({
  month: "2026-10", incomeCents: 0, expenseCents: 38000, balanceCents: -38000, dailyAverageCents: 0, byCategory: [], byEssentiality: {},
  previous: { incomeCents: 0, expenseCents: 0 },
  memberBalances: [{ userId: 1, netCents: 19000 }, { userId: 2, netCents: -19000 }],
  suggestedSettlements: [{ fromUserId: 2, toUserId: 1, amountCents: 19000 }],
  ...over,
});

const past: Settlement = { id: 3, fromUserId: 2, toUserId: 1, amountCents: 5000, date: "2026-09-20", note: "Pix do aluguel", recurrenceId: null, createdBy: 1 };

const splitting = (over = {}) => appCtx({ wallet: { ...wallet, splittingEnabled: true }, ...over });

function serve(s: Summary = summary(), history: Settlement[] = [past]) {
  return mockFetch((url, init) => {
    if (init.method && init.method !== "GET") return { body: {} };
    return { body: url.includes("/summary") ? s : history };
  });
}
const writes = (fetch: ReturnType<typeof mockFetch>) => fetch.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const writeBody = (fetch: ReturnType<typeof mockFetch>, i = 0) => bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[i]));

describe("Divisão", () => {
  it("explica e oferece ligar a divisão quando está desligada", () => {
    renderApp(<Division />);
    expect(screen.getByText(/divisão de gastos está desligada/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Ligar a divisão" }).getAttribute("href")).toBe("/ajustes?s=carteira");
  });

  it("quem não administra a carteira não vê o botão de ligar", () => {
    renderApp(<Division />, appCtx({ wallet: { ...wallet, role: "editor" } }));
    expect(screen.queryByRole("link", { name: "Ligar a divisão" })).toBeNull();
  });

  it("mostra os saldos de cada pessoa e o que falta acertar", async () => {
    serve();
    renderApp(<Division />, splitting());
    expect(await screen.findByText("tem a receber")).toBeTruthy();
    expect(screen.getByText("deve")).toBeTruthy();
    expect(screen.getByText(/\+R\$\s190,00/)).toBeTruthy();
    expect(screen.getByText(/−R\$\s190,00/)).toBeTruthy();
    expect(screen.getByText("Bia deve esse valor a você.")).toBeTruthy();
  });

  it("diz que estão quites quando não há acerto sugerido", async () => {
    serve(summary({ memberBalances: [{ userId: 1, netCents: 0 }, { userId: 2, netCents: 0 }], suggestedSettlements: [] }));
    renderApp(<Division />, splitting());
    expect(await screen.findByText("Vocês estão quites.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Registrar acerto" })).toBeNull();
  });

  it("registra o acerto sugerido com a data de hoje", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: "Registrar acerto" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements");
    expect(writeBody(fetch)).toEqual({ fromUserId: 2, toUserId: 1, amountCents: 19000, date: today() });
  });

  it("registra outro acerto à mão, validando valor e pessoas", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: "Registrar outro acerto" }));

    await user.click(screen.getByRole("button", { name: "Registrar" }));
    expect(await screen.findByText("Informe um valor maior que zero.")).toBeTruthy();

    await user.type(screen.getByLabelText("Valor do acerto"), "75,50");
    await user.selectOptions(screen.getByLabelText("Quem pagou"), "1"); // Ana paga e também recebe
    await user.click(screen.getByRole("button", { name: "Registrar" }));
    expect(await screen.findByText(/Escolha pessoas diferentes/)).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);

    await user.selectOptions(screen.getByLabelText("Quem pagou"), "2");
    await user.type(screen.getByLabelText("Observação do acerto"), "Pix");
    await user.click(screen.getByRole("button", { name: "Registrar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writeBody(fetch)).toMatchObject({ fromUserId: 2, toUserId: 1, amountCents: 7550, note: "Pix" });
  });

  it("lista o histórico com data, valor e observação", async () => {
    serve();
    renderApp(<Division />, splitting());
    expect(await screen.findByText(/Bia pagou a você/)).toBeTruthy();
    expect(screen.getByText(/Pix do aluguel/)).toBeTruthy();
    expect(screen.getByText(/R\$\s50,00/)).toBeTruthy();
  });

  it("edita um acerto do histórico com PUT", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: "Editar" }));
    const amount = screen.getByLabelText("Valor do acerto") as HTMLInputElement;
    expect(amount.value).toBe("50,00");
    await user.clear(amount);
    await user.type(amount, "60,00");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements/3");
    expect(writes(fetch)[0][1]!.method).toBe("PUT");
    expect(writeBody(fetch)).toMatchObject({ amountCents: 6000, date: "2026-09-20", note: "Pix do aluguel" });
  });

  it("exclui um acerto só depois de confirmar", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
  });

  it("leitores só consultam: sem botões de registrar, editar ou excluir", async () => {
    serve();
    renderApp(<Division />, splitting({ canEdit: false }));
    await screen.findByText(/Bia pagou a você/);
    expect(screen.queryByRole("button", { name: "Registrar acerto" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Registrar outro acerto" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Editar" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Excluir" })).toBeNull();
  });

  it("mostra erro de carregamento com opção de tentar de novo", async () => {
    mockFetch({ status: 500, body: { error: { code: "internal", message: "Erro interno" } } });
    renderApp(<Division />, splitting());
    expect(await screen.findByText("Não foi possível carregar a divisão.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeTruthy();
  });
});
