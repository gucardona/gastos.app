// Tiny stand-in for the Anthropic Messages API, used only by the e2e run (no dependencies).
import { createServer } from "node:http";

const port = Number(process.env.PORT ?? 18081);
const TEST_KEY = "sk-ant-e2e-fake-key-0000";

const sse = (res, events) => {
  res.writeHead(200, { "content-type": "text/event-stream", "cache-control": "no-cache" });
  for (const e of events) res.write(`event: ${e.type}\ndata: ${JSON.stringify(e)}\n\n`);
  res.end();
};

let calls = 0; // /v1/messages requests that got past the key check, so a test can tell a blocked question reached us or not
// the exchange's token usage is large on purpose: even the smallest limit (R$ 0,01) is crossed by one answer
const FX = { USDBRL: { code: "USD", codein: "BRL", name: "Dólar Americano/Real Brasileiro", high: "5.098", low: "5.0613", varBid: "-0.0353", pctChange: "-0.692429", bid: "5.0627", ask: "5.0631", timestamp: "1792184472", create_date: "2026-10-16 18:01:12" } };

const start = { type: "message_start", message: { id: "msg_fake", type: "message", role: "assistant", content: [], model: "fake", usage: { input_tokens: 1000, output_tokens: 0 } } };
const end = (reason) => [
  { type: "message_delta", delta: { stop_reason: reason }, usage: { output_tokens: 1000 } },
  { type: "message_stop" },
];
const text = (t) => [start,
  { type: "content_block_start", index: 0, content_block: { type: "text", text: "" } },
  { type: "content_block_delta", index: 0, delta: { type: "text_delta", text: t } },
  { type: "content_block_stop", index: 0 }, ...end("end_turn")];
const toolUse = (id, name, input) => [start,
  { type: "content_block_start", index: 0, content_block: { type: "tool_use", id, name, input: {} } },
  { type: "content_block_delta", index: 0, delta: { type: "input_json_delta", partial_json: JSON.stringify(input) } },
  { type: "content_block_stop", index: 0 }, ...end("tool_use")];

const blocks = (m) => (Array.isArray(m?.content) ? m.content : [{ type: "text", text: String(m?.content ?? "") }]);

createServer((req, res) => {
  if (req.method === "GET" && req.url === "/fx") { res.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify(FX)); return; }
  if (req.method === "GET" && req.url === "/calls") { res.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify({ calls })); return; }
  if (req.method === "GET") { res.writeHead(200).end("ok"); return; }
  let raw = "";
  req.on("data", (c) => (raw += c));
  req.on("end", () => {
    if (req.headers["x-api-key"] !== TEST_KEY) {
      res.writeHead(401, { "content-type": "application/json" });
      res.end(JSON.stringify({ type: "error", error: { type: "authentication_error", message: "invalid x-api-key" } }));
      return;
    }
    calls++;
    let body;
    try { body = JSON.parse(raw); } catch { res.writeHead(400).end("bad json"); return; }
    const msgs = body.messages ?? [];
    const last = msgs.at(-1);
    if (body.max_tokens === 1) return sse(res, text("ok"));
    if (blocks(last).some((b) => b.type === "tool_result")) {
      const prev = blocks(msgs.at(-2)).filter((b) => b.type === "tool_use").map((b) => b.name);
      if (prev.includes("mostrar_grafico")) return sse(res, text("Você gastou R$ 150,00 no total, 60% em Mercado."));
      return sse(res, toolUse("toolu_chart", "mostrar_grafico", {
        tipo: "pie", titulo: "Gastos por categoria",
        pontos: [{ rotulo: "Mercado", valor: 9000 }, { rotulo: "Lazer", valor: 6000 }],
      }));
    }
    const q = blocks(last).map((b) => b.text ?? "").join(" ");
    if (q.includes("mês")) return sse(res, toolUse("toolu_sum", "resumo_por_categoria", { de: "2026-01-01", ate: "2026-12-31" }));
    sse(res, text("Posso ajudar com os seus gastos."));
  });
}).listen(port, "127.0.0.1");
