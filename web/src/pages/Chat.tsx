import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useRef, useState } from "react";
import { Navigate, useNavigate } from "react-router";
import { api, ApiError, isUnauthenticated } from "../api/client";
import { useChat } from "../api/queries";
import type { ChatMessage } from "../api/types";
import { Chart } from "../components/Chart";
import { PageBar } from "../components/PageBar";
import { WalletPicker } from "../components/WalletPicker";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { streamChat } from "../lib/chatStream";

/** The assistant's chat, only while it is on. Keyed by the active wallet, so switching wallet starts over with only that one selected. */
export function Chat() {
  const { wallet, assistantEnabled } = useApp();
  if (!assistantEnabled) return <Navigate to="/" replace />;
  return <Conversation key={wallet.id} />;
}

function Conversation() {
  const { wallet, wallets } = useApp();
  const key = ["chat", wallet.id];
  const history = useChat(wallet.id);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const { run, busy, error } = useAction();
  const [selected, setSelected] = useState([wallet.id]);
  const [text, setText] = useState("");
  const [live, setLive] = useState<ChatMessage[]>([]);
  const [status, setStatus] = useState("");
  const [streaming, setStreaming] = useState(false);
  const abort = useRef<AbortController | null>(null);
  const end = useRef<HTMLDivElement>(null);
  const box = useRef<HTMLTextAreaElement>(null);
  const gone = useRef(false);

  useEffect(() => {
    gone.current = false; // set again because StrictMode runs the cleanup once in development
    return () => {
      gone.current = true;
      abort.current?.abort();
    };
  }, []);
  // auto-grow where CSS field-sizing is missing; max-height in the CSS still caps it
  // biome-ignore lint/correctness/useExhaustiveDependencies: measure again whenever the text changes
  useEffect(() => {
    const el = box.current;
    if (!el || globalThis.CSS?.supports?.("field-sizing", "content")) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight + el.offsetHeight - el.clientHeight}px`;
  }, [text]);
  const messages = [...(history.data ?? []), ...live];
  // biome-ignore lint/correctness/useExhaustiveDependencies: scroll on every new piece of the answer
  useEffect(() => end.current?.scrollIntoView?.({ block: "end" }), [messages.length, live]);

  async function send() {
    const q = text.trim();
    if (!q || streaming) return;
    const ctrl = new AbortController();
    abort.current = ctrl;
    const question: ChatMessage = { id: -Date.now(), role: "user", content: q, charts: [] };
    const answer: ChatMessage = { id: question.id - 1, role: "assistant", content: "", charts: [] };
    const show = () => setLive([question, { ...answer, charts: [...answer.charts] }]);
    const append = (m: string) => { answer.content += (answer.content ? "\n\n" : "") + m; show(); };
    setText("");
    setStreaming(true);
    show();
    try {
      await streamChat({ wallet: wallet.id, wallets: selected, text: q }, {
        delta: (t) => { answer.content += t; setStatus(""); show(); },
        status: setStatus,
        chart: (c) => { answer.charts.push(c); show(); },
        done: () => {},
        error: append,
      }, ctrl.signal);
    } catch (e) {
      if (gone.current) return;
      if (isUnauthenticated(e)) return navigate("/login", { replace: true });
      append(e instanceof ApiError ? (Object.values(e.fields)[0] ?? e.message) : "Sem conexão. Tente de novo.");
    }
    if (gone.current) return; // left the page or switched wallet: this thread is no longer on screen
    // the exchange joins the cached thread; the next fetch of the thread replaces it with what the server kept
    const kept = answer.content || answer.charts.length ? [question, answer] : [question];
    qc.setQueryData<ChatMessage[]>(key, (old = []) => [...old, ...kept]);
    setLive([]);
    setStatus("");
    setStreaming(false);
    abort.current = null;
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    if (streaming) abort.current?.abort();
    else void send();
  }

  const reset = () => run(async () => {
    await api.del(`/chat?wallet=${wallet.id}`);
    qc.setQueryData<ChatMessage[]>(key, []);
  });

  return (
    <div className="chat">
      <PageBar />
      <div className="chat-head">
        <WalletPicker wallets={wallets} selected={selected} onChange={setSelected} />
        {messages.length > 0 && !streaming && (
          <button type="button" className="chip" disabled={busy} onClick={() => void reset()}>Nova conversa</button>
        )}
      </div>
      {error && <p className="error" role="alert">{error}</p>}
      {messages.length > 0 && (
        <div className="chat-thread" role="log" aria-label="Conversa">
          {messages.filter((m) => m.content || m.charts.length).map((m) => (
            <div key={m.id} className={`msg msg-${m.role}`}>
              {m.content && <p>{m.content}</p>}
              {m.charts.map((c, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: charts only grow at the end and have no id
                <Chart key={i} chart={c} />
              ))}
            </div>
          ))}
          {streaming && <p className="chat-status" role="status">{status || (live[1]?.content ? "" : "Pensando…")}</p>}
          <div ref={end} />
        </div>
      )}
      <form className="composer" onSubmit={submit}>
        <textarea
          ref={box}
          aria-label="Mensagem"
          rows={1}
          maxLength={4000}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              void send();
            }
          }}
        />
        <button type="submit" className="btn">{streaming ? "Parar" : "Enviar"}</button>
      </form>
    </div>
  );
}
