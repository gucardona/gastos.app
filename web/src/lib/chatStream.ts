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
 * resolves quietly when `signal` aborts it.
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
    let buf = "";
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return;
      buf += value;
      // an event ends at a blank line; whatever follows the last one waits for the next chunk
      let end = buf.indexOf("\n\n");
      while (end >= 0) {
        dispatch(buf.slice(0, end), on);
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
  let data = "";
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) data += line.slice(5).trim();
  }
  const d = data ? JSON.parse(data) : {};
  if (event === "delta") on.delta(d.text);
  else if (event === "status") on.status(d.text);
  else if (event === "chart") on.chart(d);
  else if (event === "done") on.done();
  else if (event === "error") on.error(d.message);
}
