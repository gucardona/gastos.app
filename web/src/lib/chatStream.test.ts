import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../api/client";
import { streamChat } from "./chatStream";

/** A fetch whose response body yields `chunks`; when `hold` the stream stays open until the request is aborted. */
function sse(chunks: (string | Uint8Array)[], hold = false) {
  const fn = vi.fn(async (_url: string, init: RequestInit) => {
    const enc = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      start(c) {
        for (const ch of chunks) c.enqueue(typeof ch === "string" ? enc.encode(ch) : ch);
        if (!hold) c.close();
        init.signal?.addEventListener("abort", () => c.error(new DOMException("aborted", "AbortError")));
      },
    });
    return new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } });
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

const handlers = () => ({ delta: vi.fn(), status: vi.fn(), chart: vi.fn(), done: vi.fn(), error: vi.fn() });
const body = { wallet: 1, wallets: [1, 2], text: "Quanto gastei?" };
const chart = { kind: "pie", title: "Gastos", points: [{ label: "Mercado", value: 82000 }] };

describe("streamChat", () => {
  it("posts the question and dispatches each complete event", async () => {
    const fn = sse([
      'event: status\ndata: {"text":"Consultando os lançamentos…"}\n\n',
      'event: delta\ndata: {"text":"Você gastou "}\n\nevent: delta\ndata: {"text":"R$ 820"}\n\n',
      `event: chart\ndata: ${JSON.stringify(chart)}\n\n`,
      'event: done\ndata: {"inputTokens":10,"outputTokens":5}\n\n',
    ]);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    const [url, init] = fn.mock.calls[0];
    expect(url).toBe("/api/chat");
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("include");
    expect(JSON.parse(init.body as string)).toEqual(body);
    expect(on.status).toHaveBeenCalledWith("Consultando os lançamentos…");
    expect(on.delta.mock.calls).toEqual([["Você gastou "], ["R$ 820"]]);
    expect(on.chart).toHaveBeenCalledWith(chart);
    expect(on.done).toHaveBeenCalledTimes(1);
    expect(on.error).not.toHaveBeenCalled();
  });

  it("joins an event split across two chunks", async () => {
    sse(['event: del', 'ta\ndata: {"text":"Ol', 'á"}\n', "\nevent: done\ndata: {}\n\n"]);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.delta.mock.calls).toEqual([["Olá"]]);
    expect(on.done).toHaveBeenCalledTimes(1);
  });

  it("reads CRLF line endings, even with a \\r\\n split between chunks", async () => {
    sse(['event: delta\r\ndata: {"text":"Oi"}\r', '\n\r\nevent: done\r\ndata: {}\r\n\r\n']);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.delta.mock.calls).toEqual([["Oi"]]);
    expect(on.done).toHaveBeenCalledTimes(1);
    expect(on.error).not.toHaveBeenCalled();
  });

  it("dispatches a last event that has no blank line after it", async () => {
    sse(['event: delta\ndata: {"text":"Fim"}\n\nevent: done\ndata: {}']);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.delta).toHaveBeenCalledWith("Fim");
    expect(on.done).toHaveBeenCalledTimes(1);
  });

  it("joins several data lines with a newline", async () => {
    sse(['event: delta\ndata: {"text":\ndata: "Oi"}\n\nevent: done\ndata: {}\n\n']);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.delta).toHaveBeenCalledWith("Oi");
  });

  it("decodes a multi-byte character split across two chunks", async () => {
    const bytes = new TextEncoder().encode('event: delta\ndata: {"text":"ç"}\n\nevent: done\ndata: {}\n\n');
    const cut = bytes.indexOf(0xc3) + 1; // between the two bytes of "ç"
    sse([bytes.slice(0, cut), bytes.slice(cut)]);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.delta).toHaveBeenCalledWith("ç");
  });

  it("skips a malformed event and keeps reading", async () => {
    sse(['event: delta\ndata: {"text":\n\n', 'event: delta\ndata: {"text":"ok"}\n\nevent: done\ndata: {}\n\n']);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.delta.mock.calls).toEqual([["ok"]]);
    expect(on.error).not.toHaveBeenCalled();
  });

  it("a stream that ends without done or error reports the answer as interrupted", async () => {
    sse(['event: delta\ndata: {"text":"Parcial"}\n\n']);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.error).toHaveBeenCalledWith("Resposta interrompida.");
  });

  it("an error event calls on.error with its message", async () => {
    sse(['event: error\ndata: {"message":"A Anthropic recusou a chave. Informe a chave de novo em Ajustes."}\n\n']);
    const on = handlers();
    await streamChat(body, on, new AbortController().signal);
    expect(on.error).toHaveBeenCalledWith("A Anthropic recusou a chave. Informe a chave de novo em Ajustes.");
  });

  it("aborting resolves without throwing", async () => {
    sse(['event: delta\ndata: {"text":"Parcial"}\n\n'], true);
    const on = handlers();
    const ctrl = new AbortController();
    const p = streamChat(body, on, ctrl.signal);
    await vi.waitFor(() => expect(on.delta).toHaveBeenCalledWith("Parcial"));
    ctrl.abort();
    await expect(p).resolves.toBeUndefined();
    expect(on.error).not.toHaveBeenCalled();
  });

  it("a refused request throws an ApiError with the server's message, as the api helper does", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "conflict", message: "Aguarde a resposta anterior terminar." } }), { status: 409 })));
    const err = await streamChat(body, handlers(), new AbortController().signal).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(409);
    expect(err.message).toBe("Aguarde a resposta anterior terminar.");
  });
});
