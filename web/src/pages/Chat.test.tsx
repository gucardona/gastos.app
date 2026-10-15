import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import type { ChatMessage } from "../api/types";
import { Ctx } from "../context";
import { appCtx, wallet } from "../test/fixtures";
import { Chat } from "./Chat";

const viagem = { ...wallet, id: 2, name: "Viagem" };

/**
 * Fakes the chat API. POST /api/chat answers with a stream the test feeds through `push` and ends with `end`;
 * the stream fails when the request is aborted, as the browser's does.
 */
function fakeApi(history: ChatMessage[] = [], refuse?: { status: number; body: unknown }) {
  const enc = new TextEncoder();
  let ctl: ReadableStreamDefaultController<Uint8Array> | null = null;
  const posts: { body: unknown; signal: AbortSignal }[] = [];
  const fn = vi.fn(async (url: string, init: RequestInit = {}) => {
    const method = init.method ?? "GET";
    if (url.startsWith("/api/chat?") && method === "GET") return Response.json(history);
    if (url.startsWith("/api/chat?") && method === "DELETE") return new Response(null, { status: 204 });
    if (url === "/api/chat" && method === "POST" && refuse) return Response.json(refuse.body, { status: refuse.status });
    if (url === "/api/chat" && method === "POST") {
      const signal = init.signal as AbortSignal;
      posts.push({ body: JSON.parse(init.body as string), signal });
      const body = new ReadableStream<Uint8Array>({
        start(c) {
          ctl = c;
          signal.addEventListener("abort", () => c.error(new DOMException("aborted", "AbortError")));
        },
      });
      return new Response(body, { status: 200 });
    }
    return new Response("{}", { status: 404 });
  });
  vi.stubGlobal("fetch", fn);
  return {
    fn,
    posts,
    push: (event: string, data: unknown) => ctl?.enqueue(enc.encode(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`)),
    end: () => ctl?.close(),
    fail: () => ctl?.error(new TypeError("network error")),
  };
}

function renderChat(ctx = appCtx({ assistantEnabled: true, wallets: [wallet, viagem] })) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const ui = (c: typeof ctx) => (
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={["/chat"]}>
        <Ctx.Provider value={c}>
          <Routes>
            <Route path="/chat" element={<Chat />} />
            <Route path="/login" element={<p>Entrar</p>} />
          </Routes>
        </Ctx.Provider>
      </MemoryRouter>
    </QueryClientProvider>
  );
  const r = render(ui(ctx));
  return { ...r, rerenderWith: (c: typeof ctx) => r.rerender(ui(c)) };
}

const input = () => screen.getByRole("textbox", { name: "Mensagem" });
const thread = () => screen.getByRole("log", { name: "Conversa" });

describe("Chat", () => {
  it("asks only about the active wallet by default", async () => {
    const api = fakeApi();
    renderChat();
    expect(screen.getByRole("button", { name: /Casa/ })).toBeTruthy();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    expect(api.posts[0].body).toEqual({ wallet: 1, wallets: [1], text: "Quanto gastei?" });
  });

  it("Enter sends, Shift+Enter breaks the line", async () => {
    const api = fakeApi();
    renderChat();
    await userEvent.type(input(), "Linha um{Shift>}{Enter}{/Shift}linha dois");
    expect(api.posts).toHaveLength(0);
    expect((input() as HTMLTextAreaElement).value).toBe("Linha um\nlinha dois");
    await userEvent.keyboard("{Enter}");
    expect(api.posts[0].body).toMatchObject({ text: "Linha um\nlinha dois" });
    expect((input() as HTMLTextAreaElement).value).toBe("");
  });

  it("shows the question on the right, then the answer as it arrives, the status line and the chart", async () => {
    const api = fakeApi();
    renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    expect(within(thread()).getByText("Quanto gastei?").closest(".msg")?.className).toContain("msg-user");
    api.push("status", { text: "Consultando os lançamentos…" });
    expect(await screen.findByText("Consultando os lançamentos…")).toBeTruthy();
    api.push("delta", { text: "Você gastou " });
    api.push("delta", { text: "R$ 820." });
    const answer = await within(thread()).findByText("Você gastou R$ 820.");
    expect(answer.closest(".msg")?.className).toContain("msg-assistant");
    expect(screen.queryByText("Consultando os lançamentos…")).toBeNull();
    api.push("chart", { kind: "pie", title: "Gastos do mês", points: [{ label: "Mercado", value: 82000 }] });
    expect(await within(thread()).findByRole("img", { name: /Gastos do mês/ })).toBeTruthy();
    api.push("done", {});
    api.end();
    expect(await screen.findByRole("button", { name: "Enviar" })).toBeTruthy();
  });

  it("the send button turns into Parar while answering, and Parar aborts the request", async () => {
    const api = fakeApi();
    renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    api.push("delta", { text: "Parcial" });
    await within(thread()).findByText("Parcial");
    await userEvent.click(screen.getByRole("button", { name: "Parar" }));
    expect(api.posts[0].signal.aborted).toBe(true);
    expect(await screen.findByRole("button", { name: "Enviar" })).toBeTruthy();
    expect(within(thread()).getByText("Parcial")).toBeTruthy();
  });

  it("Nova conversa deletes the thread and clears the screen", async () => {
    const api = fakeApi([
      { id: 1, role: "user", content: "Quanto gastei?", charts: [] },
      { id: 2, role: "assistant", content: "R$ 820.", charts: [] },
    ]);
    renderChat();
    expect(await screen.findByText("R$ 820.")).toBeTruthy();
    expect(within(thread()).getByText("Quanto gastei?")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Nova conversa" }));
    expect(api.fn).toHaveBeenCalledWith("/api/chat?wallet=1", expect.objectContaining({ method: "DELETE" }));
    await vi.waitFor(() => expect(screen.queryByText("R$ 820.")).toBeNull());
    expect(screen.queryByText("Quanto gastei?")).toBeNull();
  });

  it("an error from the server shows as the consultant's message", async () => {
    const api = fakeApi();
    renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    api.push("error", { message: "A conta da Anthropic está sem saldo. Adicione créditos e tente de novo." });
    api.end();
    const msg = await within(thread()).findByText("A conta da Anthropic está sem saldo. Adicione créditos e tente de novo.");
    expect(msg.closest(".msg")?.className).toContain("msg-assistant");
  });

  it("a lost connection after part of the answer keeps that part and adds the notice below it", async () => {
    const api = fakeApi();
    renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    api.push("delta", { text: "Parcial" });
    await within(thread()).findByText("Parcial");
    api.fail();
    await vi.waitFor(() => expect(thread().querySelector(".msg-assistant")?.textContent).toBe("Parcial\n\nSem conexão. Tente de novo."));
  });

  it("an answer cut without done or error says it was interrupted", async () => {
    const api = fakeApi();
    renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    api.push("delta", { text: "Parcial" });
    api.end();
    await vi.waitFor(() => expect(thread().querySelector(".msg-assistant")?.textContent).toBe("Parcial\n\nResposta interrompida."));
  });

  it.each([
    [409, "Aguarde a resposta anterior terminar."],
    [429, "Muitas mensagens seguidas, aguarde um minuto."],
  ])("a %i before the stream shows the server's message as the consultant's", async (status, message) => {
    fakeApi([], { status, body: { error: { code: "x", message } } });
    renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    const msg = await within(thread()).findByText(message);
    expect(msg.closest(".msg")?.className).toContain("msg-assistant");
  });

  it("a 401 goes to the login", async () => {
    fakeApi([], { status: 401, body: { error: { code: "unauthenticated", message: "Faça login." } } });
    renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    expect(await screen.findByText("Entrar")).toBeTruthy();
  });

  it("leaving the page aborts the request", async () => {
    const api = fakeApi();
    const { unmount } = renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    await vi.waitFor(() => expect(api.posts).toHaveLength(1));
    unmount();
    expect(api.posts[0].signal.aborted).toBe(true);
  });

  it("switching wallet mid-answer aborts it and leaves the other thread untouched", async () => {
    const api = fakeApi();
    const { rerenderWith } = renderChat();
    await userEvent.type(input(), "Quanto gastei?{Enter}");
    api.push("delta", { text: "Parcial" });
    await within(thread()).findByText("Parcial");
    rerenderWith(appCtx({ assistantEnabled: true, wallet: viagem, wallets: [wallet, viagem] }));
    expect(api.posts[0].signal.aborted).toBe(true);
    await vi.waitFor(() => expect(screen.getByRole("button", { name: "Enviar" })).toBeTruthy());
    expect(screen.queryByText("Parcial")).toBeNull();
  });

  it("an empty conversation shows only the text field: no suggestions, no empty-state text", async () => {
    const api = fakeApi();
    const { container } = renderChat();
    await vi.waitFor(() => expect(api.fn).toHaveBeenCalled());
    expect(screen.queryByRole("log")).toBeNull();
    expect(container.querySelector(".empty")).toBeNull();
    expect(screen.queryByRole("button", { name: "Nova conversa" })).toBeNull();
    expect(input()).toBeTruthy();
  });

  it("switching the active wallet resets the selection to that wallet", async () => {
    const api = fakeApi();
    const { rerenderWith } = renderChat();
    await userEvent.click(screen.getByRole("button", { name: /Casa/ }));
    await userEvent.click(screen.getByRole("button", { name: "Selecionar todas" }));
    await userEvent.click(screen.getByRole("button", { name: "Fechar" }));
    expect(screen.getByRole("button", { name: /2 carteiras/ })).toBeTruthy();
    rerenderWith(appCtx({ assistantEnabled: true, wallet: viagem, wallets: [wallet, viagem] }));
    expect(screen.getByRole("button", { name: /Viagem/ })).toBeTruthy();
    await userEvent.type(input(), "E aqui?{Enter}");
    expect(api.posts[0].body).toEqual({ wallet: 2, wallets: [2], text: "E aqui?" });
  });
});
