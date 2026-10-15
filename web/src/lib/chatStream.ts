import { errorOf } from "../api/client";
import type { ChartData } from "../api/types";

export interface ChatHandlers {
  delta(t: string): void;
  status(t: string): void;
  chart(c: ChartData): void;
  done(): void;
  error(m: string): void;
}

/**
 * Posts a question and reads the SSE answer as it streams. Throws an ApiError when the server refuses the request;
 * resolves quietly when `signal` aborts it. A stream that ends without `done` or `error` reports "Resposta interrompida.".
 */
export async function streamChat(body: { wallet: number; wallets: number[]; text: string }, on: ChatHandlers, signal: AbortSignal): Promise<void> {
  try {
    const res = await fetch("/api/chat", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    });
    if (!res.ok || !res.body) throw await errorOf(res);
    const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
    let finished = false;
    const handlers: ChatHandlers = {
      ...on,
      done: () => { finished = true; on.done(); },
      error: (m) => { finished = true; on.error(m); },
    };
    let buf = "";
    let carry = ""; // a "\r" at the end of a chunk may be the first half of "\r\n"
    for (;;) {
      const { value, done } = await reader.read();
      if (done) {
        buf += carry.replace("\r", "\n");
        if (buf.trim()) dispatch(buf, handlers); // the last event may lack its blank line
        if (!finished) on.error("Resposta interrompida.");
        return;
      }
      let text = carry + value;
      carry = text.endsWith("\r") ? "\r" : "";
      if (carry) text = text.slice(0, -1);
      buf += text.replace(/\r\n?/g, "\n");
      // an event ends at a blank line; whatever follows the last one waits for the next chunk
      let end = buf.indexOf("\n\n");
      while (end >= 0) {
        dispatch(buf.slice(0, end), handlers);
        buf = buf.slice(end + 2);
        end = buf.indexOf("\n\n");
      }
    }
  } catch (e) {
    if (signal.aborted) return;
    throw e;
  }
}

function dispatch(block: string, on: ChatHandlers) {
  let event = "message";
  const data: string[] = [];
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) data.push(line.slice(5).replace(/^ /, ""));
  }
  let d: { text?: string; message?: string };
  try {
    d = data.length ? JSON.parse(data.join("\n")) : {};
  } catch {
    return; // one malformed event is skipped, not the whole answer
  }
  if (event === "delta") on.delta(d.text ?? "");
  else if (event === "status") on.status(d.text ?? "");
  else if (event === "chart") on.chart(d as unknown as ChartData);
  else if (event === "done") on.done();
  else if (event === "error") on.error(d.message ?? "Algo deu errado. Tente de novo.");
}
