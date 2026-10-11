import { useState } from "react";
import { Link } from "react-router";
import { api } from "../api/client";
import { useInbox } from "../api/queries";
import type { InboxItem } from "../api/types";
import { PageBar } from "../components/PageBar";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { formatBRL } from "../lib/money";

const shortDate = (iso: string) => new Date(`${iso}T12:00:00`).toLocaleDateString("pt-BR", { day: "numeric", month: "short" });

export function Inbox() {
  const { wallet, canEdit } = useApp();
  const inbox = useInbox(wallet.id);
  const { run, busy, error } = useAction();
  const safe = inbox.data?.filter((i) => i.suggestedCategoryId && !i.suggestedMatch).length ?? 0;

  return (
    <>
      <PageBar />
      <h1 style={{ font: "700 28px var(--display)", margin: "8px 0 4px" }}>Caixa de entrada</h1>
      <p className="muted" style={{ margin: "0 0 12px" }}>Linhas dos extratos importados. Aceite para criar o lançamento, ou ignore o que não interessa.</p>
      {canEdit && safe > 0 && (
        <button type="button" className="btn" disabled={busy} onClick={() => void run(() => api.post(`/wallets/${wallet.id}/inbox/accept-suggested`))}>
          Aceitar {safe} {safe === 1 ? "sugerida" : "sugeridas"}
        </button>
      )}
      {error && <p className="error" role="alert">{error}</p>}
      {inbox.error ? (
        <div className="empty"><p>Não foi possível carregar a caixa de entrada.</p><button type="button" className="btn" onClick={() => void inbox.refetch()}>Tentar de novo</button></div>
      ) : !inbox.data ? (
        <><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></>
      ) : inbox.data.length === 0 ? (
        <div className="empty">
          <p>Nada para revisar. Importe um extrato OFX ou CSV em <Link to="/ajustes?s=bancos">Ajustes, Contas bancárias</Link>.</p>
        </div>
      ) : (
        inbox.data.map((i) => <Line key={i.id} item={i} />)
      )}
    </>
  );
}

function Line({ item }: { item: InboxItem }) {
  const { wallet, categories, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const kind = item.amountCents > 0 ? "income" : "expense";
  const options = categories.filter((c) => c.kind === kind && (!c.archived || c.id === item.suggestedCategoryId));
  const [categoryId, setCategoryId] = useState(item.suggestedCategoryId ?? 0);
  const [remember, setRemember] = useState(false);
  const [pattern, setPattern] = useState(() => item.description.toLowerCase().split(/\s+/).slice(0, 2).join(" "));
  const base = `/wallets/${wallet.id}/inbox/${item.id}`;
  const title = item.suggestedDescription || item.description;
  const offerRule = !item.suggestedCategoryId && categoryId > 0;

  const accept = () =>
    run(async () => {
      await api.post(`${base}/accept`, { categoryId });
      if (offerRule && remember && pattern.trim()) await api.post(`/wallets/${wallet.id}/rules`, { pattern, categoryId });
    });

  return (
    <div className="item">
      <div className="row" style={{ borderTop: 0, minHeight: 0, padding: 0 }}>
        <span className="emoji" aria-hidden>{kind === "income" ? "＋" : "−"}</span>
        <span>
          <span className="desc">{title}</span>
          <span className="meta" style={{ display: "block" }}>
            {shortDate(item.date)}, {item.accountName}
            {item.suggestedDescription && <><br />No extrato: {item.description}</>}
          </span>
        </span>
        <span className={`val num ${kind === "income" ? "in" : ""}`}>{kind === "income" ? "+" : "−"}{formatBRL(Math.abs(item.amountCents))}</span>
      </div>
      {item.suggestedMatch && (
        <p className="meta" style={{ margin: "8px 0 0" }}>
          Parece o lançamento que você já fez: {item.suggestedMatch.description}, {shortDate(item.suggestedMatch.date)}.
        </p>
      )}
      {canEdit && (
        <div className="inline-form" style={{ marginTop: 10 }}>
          <label>Categoria
            <select aria-label={`Categoria de ${title}`} value={categoryId} onChange={(e) => setCategoryId(Number(e.target.value))}>
              <option value={0}>Escolha</option>
              {options.map((c) => <option key={c.id} value={c.id}>{c.icon} {c.name}</option>)}
            </select>
          </label>
          {offerRule && (
            <>
              <label className="inline"><input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} /> Usar esta categoria nas próximas importações</label>
              {remember && <label>Quando a descrição tiver<input className="text" aria-label="Texto da regra" value={pattern} onChange={(e) => setPattern(e.target.value)} /></label>}
            </>
          )}
          <div className="item-actions">
            <button type="button" className="btn" disabled={busy || categoryId === 0} onClick={() => void accept()}>Aceitar</button>
            {item.suggestedMatch && (
              <button type="button" className="chip" disabled={busy} onClick={() => void run(() => api.post(`${base}/match`, { transactionId: item.suggestedMatch!.id }))}>É este lançamento</button>
            )}
            <button type="button" className="chip" disabled={busy} onClick={() => void run(() => api.post(`${base}/ignore`))}>Ignorar</button>
          </div>
        </div>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}
