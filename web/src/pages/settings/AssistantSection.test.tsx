import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { AISettings } from "../../api/types";
import { bodyOf, mockFetch, renderApp } from "../../test/fixtures";
import { AssistantSection } from "./AssistantSection";

const KEY = "sk-ant-test-key-1234";
const base: AISettings = { available: true, configured: false, keyHint: "", model: "claude-sonnet-5-5", enabled: false, retentionDays: 7,
  dailyLimitCents: 0, monthlyLimitCents: 0, usedTodayCents: 0, usedMonthCents: 0, usdBrl: null, usdBrlAt: null };
const withKey: AISettings = { ...base, configured: true, keyHint: "1234" };

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));

/** Serves GET /me/ai from `state`; a PUT merges its body into it, like the API does. */
function serve(initial: AISettings, putReply?: { status: number; body: unknown }) {
  const state = { ...initial };
  const fetch = mockFetch((url, init) => {
    if (init.method === "PUT") {
      if (putReply) return putReply;
      const b = JSON.parse(init.body as string);
      if (b.apiKey) Object.assign(state, { configured: true, keyHint: b.apiKey.slice(-4) });
      Object.assign(state, { ...b, apiKey: undefined });
      return { body: state };
    }
    if (init.method === "DELETE") { Object.assign(state, { configured: false, keyHint: "", enabled: false }); return { status: 204 }; }
    return { body: url === "/api/me/ai" ? state : {} };
  });
  return fetch;
}

describe("Assistant screen", () => {
  it("when the server has no master key it only explains that", async () => {
    mockFetch({ body: { ...base, available: false } });
    renderApp(<AssistantSection />);
    expect(await screen.findByText(/não tem a chave mestra do assistente/)).toBeTruthy();
    expect(screen.queryByLabelText("Chave da API")).toBeNull();
    expect(screen.queryByRole("switch")).toBeNull();
  });

  it("saves the key, sends only { apiKey }, and afterwards shows just its end", async () => {
    const fetch = serve(base);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    const input = (await screen.findByLabelText("Chave da API")) as HTMLInputElement;
    expect(input.type).toBe("password");
    expect(input.autocomplete).toBe("off");
    await user.type(input, KEY);
    await user.click(screen.getByRole("button", { name: "Salvar chave" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/me/ai");
    expect(bodyAt(fetch)).toEqual({ apiKey: KEY });
    expect(await screen.findByText("•••• 1234")).toBeTruthy();
    expect(screen.queryByLabelText("Chave da API")).toBeNull();
    expect(document.body.textContent).not.toContain(KEY);
  });

  it("Remover deletes the key", async () => {
    const fetch = serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.click(await screen.findByRole("button", { name: "Remover" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/me/ai/key");
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
    expect(await screen.findByLabelText("Chave da API")).toBeTruthy();
  });

  it("Trocar opens the key field again", async () => {
    serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.click(await screen.findByRole("button", { name: "Trocar" }));
    expect(screen.getByLabelText("Chave da API")).toBeTruthy();
  });

  it("changing the model sends { model }", async () => {
    const fetch = serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.click(await screen.findByRole("button", { name: "Opus" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toEqual({ model: "claude-opus-5-5" });
  });

  it("the chat switch sends { enabled }, and is disabled without a key", async () => {
    const fetch = serve(withKey);
    const user = userEvent.setup();
    const { unmount } = renderApp(<AssistantSection />);
    await user.click(await screen.findByRole("switch", { name: "Ativar chat" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toEqual({ enabled: true });
    unmount();

    serve(base);
    renderApp(<AssistantSection />);
    expect(((await screen.findByRole("switch", { name: "Ativar chat" })) as HTMLButtonElement).disabled).toBe(true);
  });

  it("a retention outside 1 to 30 shows an error and sends nothing", async () => {
    const fetch = serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    const input = await screen.findByLabelText("Guardar conversas por (dias)");
    await user.clear(input);
    await user.type(input, "45");
    await user.click(screen.getByRole("button", { name: "Salvar prazo" }));
    expect(await screen.findByText("Use de 1 a 30 dias.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);
  });

  it("a valid retention is sent as a number", async () => {
    const fetch = serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    const input = await screen.findByLabelText("Guardar conversas por (dias)");
    await user.clear(input);
    await user.type(input, "14");
    await user.click(screen.getByRole("button", { name: "Salvar prazo" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toEqual({ retentionDays: 14 });
  });

  it("shows the server error for the key under the field", async () => {
    serve(base, { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { apiKey: "chave recusada pela Anthropic" } } } });
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.type(await screen.findByLabelText("Chave da API"), KEY);
    await user.click(screen.getByRole("button", { name: "Salvar chave" }));
    expect(await screen.findByText("chave recusada pela Anthropic")).toBeTruthy();
  });

  it("shows the warning when the key was saved without being validated", async () => {
    serve(base, { status: 200, body: { ...withKey, warning: "Não foi possível validar a chave agora; ela foi salva mesmo assim." } });
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.type(await screen.findByLabelText("Chave da API"), KEY);
    await user.click(screen.getByRole("button", { name: "Salvar chave" }));
    expect(await screen.findByText(/ela foi salva mesmo assim/)).toBeTruthy();
  });

  it("shows the privacy notice", async () => {
    serve(base);
    renderApp(<AssistantSection />);
    expect(await screen.findByText("Os dados dos lançamentos consultados são enviados à Anthropic com a sua chave.")).toBeTruthy();
  });

  it("shows the asterisks and the exact footnote even with no limit set", async () => {
    serve(withKey);
    renderApp(<AssistantSection />);
    expect(await screen.findByLabelText("Limite diário (R$) *")).toBeTruthy();
    expect(screen.getByLabelText("Limite mensal (R$) *")).toBeTruthy();
    expect(screen.getByText("* Valor aproximado: o custo é estimado com a cotação do dólar e pode variar um pouco do que a Anthropic cobra. O limite real é o da sua conta no console da Anthropic.")).toBeTruthy();
  });

  it.each([
    ["5", 500],
    ["5,00", 500],
    ["5.50", 550],
    ["5,5", 550],
    ["", 0],
    ["0", 0],
  ])("saving a daily limit of %j sends { dailyLimitCents: %i }", async (text, cents) => {
    const fetch = serve({ ...withKey, dailyLimitCents: cents === 0 ? 300 : 0 });
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    const input = await screen.findByLabelText("Limite diário (R$) *");
    await user.clear(input);
    if (text) await user.type(input, text);
    await user.click(screen.getByRole("button", { name: "Salvar limite diário" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toEqual({ dailyLimitCents: cents });
  });

  it("the monthly limit is sent on its own", async () => {
    const fetch = serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.type(await screen.findByLabelText("Limite mensal (R$) *"), "100");
    await user.click(screen.getByRole("button", { name: "Salvar limite mensal" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toEqual({ monthlyLimitCents: 10000 });
  });

  it("the save chip only appears once the value changed", async () => {
    serve({ ...withKey, dailyLimitCents: 500 });
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    const input = (await screen.findByLabelText("Limite diário (R$) *")) as HTMLInputElement;
    expect(input.value).toBe("5,00");
    expect(screen.queryByRole("button", { name: "Salvar limite diário" })).toBeNull();
    await user.type(input, "1");
    expect(screen.getByRole("button", { name: "Salvar limite diário" })).toBeTruthy();
  });

  it.each(["abc", "-5", "100000,01", "1,234"])("an invalid limit %j shows an error and sends nothing", async (text) => {
    const fetch = serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.type(await screen.findByLabelText("Limite diário (R$) *"), text);
    await user.click(screen.getByRole("button", { name: "Salvar limite diário" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Use um valor de R$ 0,01 a R$ 100.000,00, ou deixe vazio para não ter limite.");
    expect(writes(fetch)).toHaveLength(0);
  });


  it("links both limit inputs to the footnote", async () => {
    serve(withKey);
    renderApp(<AssistantSection />);
    const note = (await screen.findByText(/^\* Valor aproximado/)).id;
    expect(note).not.toBe("");
    expect(screen.getByLabelText("Limite diário (R$) *").getAttribute("aria-describedby")).toBe(note);
    expect(screen.getByLabelText("Limite mensal (R$) *").getAttribute("aria-describedby")).toBe(note);
  });

  it("a valid edit of one limit does not clear the other limit's error", async () => {
    serve(withKey);
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.type(await screen.findByLabelText("Limite diário (R$) *"), "abc");
    await user.click(screen.getByRole("button", { name: "Salvar limite diário" }));
    expect(await screen.findByRole("alert")).toBeTruthy();
    await user.type(screen.getByLabelText("Limite mensal (R$) *"), "100");
    await user.click(screen.getByRole("button", { name: "Salvar limite mensal" }));
    await waitFor(() => expect(screen.getByLabelText("Limite mensal (R$) *")).toBeTruthy());
    expect(screen.getByRole("alert").textContent).toContain("R$ 0,01 a R$ 100.000,00");
  });
  it("shows the server's error for a limit", async () => {
    serve(withKey, { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { dailyLimitCents: "limite fora do intervalo" } } } });
    const user = userEvent.setup();
    renderApp(<AssistantSection />);
    await user.type(await screen.findByLabelText("Limite diário (R$) *"), "5");
    await user.click(screen.getByRole("button", { name: "Salvar limite diário" }));
    expect(await screen.findByText("limite fora do intervalo")).toBeTruthy();
  });

  it("shows today's and this month's usage against the limits", async () => {
    serve({ ...withKey, dailyLimitCents: 500, monthlyLimitCents: 10000, usedTodayCents: 12, usedMonthCents: 345 });
    renderApp(<AssistantSection />);
    expect(await screen.findByText("Hoje: R$ 0,12 de R$ 5,00")).toBeTruthy();
    expect(screen.getByText("Este mês: R$ 3,45 de R$ 100,00")).toBeTruthy();
  });

  it("shows just the usage when there is no limit", async () => {
    serve({ ...withKey, usedTodayCents: 12, usedMonthCents: 345 });
    renderApp(<AssistantSection />);
    expect(await screen.findByText("Hoje: R$ 0,12")).toBeTruthy();
    expect(screen.getByText("Este mês: R$ 3,45")).toBeTruthy();
  });

  it("shows the rate in use and when it was updated", async () => {
    serve({ ...withKey, usdBrl: 5.0631, usdBrlAt: "2026-10-16T15:00:00Z" });
    renderApp(<AssistantSection />);
    expect(await screen.findByText("Cotação usada: US$ 1 = R$ 5,06 (atualizada em 16/10)")).toBeTruthy();
  });

  it("says so when there is no rate yet", async () => {
    serve(withKey);
    renderApp(<AssistantSection />);
    expect(await screen.findByText("Cotação do dólar indisponível: os limites em reais só passam a valer quando ela for obtida.")).toBeTruthy();
  });
});
