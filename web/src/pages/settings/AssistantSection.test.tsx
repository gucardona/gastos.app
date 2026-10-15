import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { AISettings } from "../../api/types";
import { bodyOf, mockFetch, renderApp } from "../../test/fixtures";
import { AssistantSection } from "./AssistantSection";

const KEY = "sk-ant-test-key-1234";
const base: AISettings = { available: true, configured: false, keyHint: "", model: "claude-sonnet-5-5", enabled: false, retentionDays: 7 };
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
});
