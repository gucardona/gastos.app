import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Member, Settlement, Summary } from "../api/types";
import { today } from "../lib/dates";
import { appCtx, bodyOf, members, mockFetch, renderApp, wallet } from "../test/fixtures";
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

describe("Division", () => {
  it("explains and offers to turn splitting on when it is off", () => {
    renderApp(<Division />);
    expect(screen.getByText(/divisão de gastos está desligada/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Ligar a divisão" }).getAttribute("href")).toBe("/ajustes/carteira");
  });

  it("whoever does not manage the wallet does not see the turn-on button", () => {
    renderApp(<Division />, appCtx({ wallet: { ...wallet, role: "editor" } }));
    expect(screen.queryByRole("link", { name: "Ligar a divisão" })).toBeNull();
  });

  it("says who owes whom in one sentence, and each person's balance", async () => {
    serve();
    renderApp(<Division />, splitting());
    expect(await screen.findByText(/Bia deve R\$\s190,00 a você\./)).toBeTruthy();
    expect(screen.getByText("tem a receber")).toBeTruthy();
    expect(screen.getByText("deve")).toBeTruthy();
    expect(screen.getByText(/\+R\$\s190,00/)).toBeTruthy();
    expect(screen.getByText(/−R\$\s190,00/)).toBeTruthy();
  });

  it("says 'Você' when it is you who owes", async () => {
    serve(summary({ suggestedSettlements: [{ fromUserId: 1, toUserId: 2, amountCents: 4000 }] }));
    renderApp(<Division />, splitting());
    expect(await screen.findByText(/Você deve R\$\s40,00 para Bia\./)).toBeTruthy();
  });

  it("says everyone is even when there is no suggested settlement", async () => {
    serve(summary({ memberBalances: [{ userId: 1, netCents: 0 }, { userId: 2, netCents: 0 }], suggestedSettlements: [] }));
    renderApp(<Division />, splitting());
    expect(await screen.findByText("Vocês estão quites.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Registrar acerto" })).toBeNull();
  });

  it("with three people each suggested transfer has its own sentence and button", async () => {
    const caio: Member = { userId: 3, name: "Caio Dias", email: "caio@example.com", role: "editor", splitsByDefault: true };
    const user = userEvent.setup();
    const fetch = mockFetch((url, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: url.includes("/summary") ? summary({ suggestedSettlements: [{ fromUserId: 2, toUserId: 1, amountCents: 10000 }, { fromUserId: 3, toUserId: 1, amountCents: 9000 }] }) : [] }));
    renderApp(<Division />, splitting({ members: [...members, caio] }));
    expect(await screen.findByText(/Bia deve R\$\s100,00 a você\./)).toBeTruthy();
    expect(screen.getByText(/Caio deve R\$\s90,00 a você\./)).toBeTruthy();
    const buttons = screen.getAllByRole("button", { name: "Registrar acerto" });
    expect(buttons).toHaveLength(2);
    await user.click(buttons[1]);
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writeBody(fetch)).toEqual({ fromUserId: 3, toUserId: 1, amountCents: 9000, date: today() });
  });

  it("registers the suggested settlement with today's date", async () => {
    const fetch = serve();
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: "Registrar acerto" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/settlements");
    expect(writeBody(fetch)).toEqual({ fromUserId: 2, toUserId: 1, amountCents: 19000, date: today() });
  });

  it("shows the error when registering the suggestion fails", async () => {
    mockFetch((url, init) => (init.method === "POST" ? { status: 500, body: { error: { code: "internal", message: "Erro interno" } } } : { body: url.includes("/summary") ? summary() : [] }));
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: "Registrar acerto" }));
    expect(await screen.findByText("Erro interno")).toBeTruthy();
  });

  it("'Registrar outro acerto' opens the sheet to register one by hand", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: "Registrar outro acerto" }));
    expect(screen.getByLabelText("Valor do acerto")).toBeTruthy();
  });

  it("lists the history with the people, the date, the amount and the note", async () => {
    serve();
    renderApp(<Division />, splitting());
    expect(await screen.findByText(/Bia pagou a você/)).toBeTruthy();
    expect(screen.getByText(/Pix do aluguel/)).toBeTruthy();
    expect(screen.getByText(/R\$\s50,00/)).toBeTruthy();
  });

  it("tapping a settlement of the history opens it for editing", async () => {
    serve();
    const user = userEvent.setup();
    renderApp(<Division />, splitting());
    await user.click(await screen.findByRole("button", { name: /Bia pagou a você/ }));
    expect((screen.getByLabelText("Valor do acerto") as HTMLInputElement).value).toBe("50,00");
  });

  it("with no history it says so", async () => {
    serve(summary(), []);
    renderApp(<Division />, splitting());
    expect(await screen.findByText("Nenhum acerto registrado ainda.")).toBeTruthy();
  });

  it("readers only look: no register and no tappable history", async () => {
    serve();
    renderApp(<Division />, splitting({ canEdit: false }));
    await screen.findByText(/Bia pagou a você/);
    expect(screen.queryByRole("button", { name: "Registrar acerto" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Registrar outro acerto" })).toBeNull();
    expect(screen.queryByRole("button", { name: /Bia pagou a você/ })).toBeNull();
  });

  it("shows a load error with a retry option", async () => {
    mockFetch({ status: 500, body: { error: { code: "internal", message: "Erro interno" } } });
    renderApp(<Division />, splitting());
    expect(await screen.findByText("Não foi possível carregar a divisão.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeTruthy();
  });
});
