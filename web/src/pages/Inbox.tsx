import { useState } from "react";
import { Link } from "react-router";
import { api } from "../api/client";
import { useInbox } from "../api/queries";
import type { InboxItem } from "../api/types";
import { InboxSheet } from "../components/InboxSheet";
import { ListGroup } from "../components/ListGroup";
import { PageBar } from "../components/PageBar";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { formatBRL } from "../lib/money";

const shortDate = (iso: string) => new Date(`${iso}T12:00:00`).toLocaleDateString("pt-BR", { day: "numeric", month: "short" });

export function Inbox() {
  const { wallet, canEdit } = useApp();
  const inbox = useInbox(wallet.id);
  const { run, busy, error } = useAction();
  const [selected, setSelected] = useState<InboxItem | null>(null);
  const safe = inbox.data?.filter((i) => i.suggestedCategoryId && !i.suggestedMatch).length ?? 0;

  return (
    <>
      <PageBar />
      <h1 className="page-title">Caixa de entrada</h1>
      <p className="muted lead">Linhas dos extratos importados. Toque numa linha para criar o lançamento, ligar a um que você já fez, ou ignorar.</p>
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
        <ListGroup className="inbox-list">
          {inbox.data.map((i) => <Line key={i.id} item={i} onOpen={canEdit ? () => setSelected(i) : undefined} />)}
        </ListGroup>
      )}
      <InboxSheet item={selected} onClose={() => setSelected(null)} />
    </>
  );
}

function Line({ item, onOpen }: { item: InboxItem; onOpen?: () => void }) {
  const income = item.amountCents > 0;
  const content = (
    <>
      <span className="lr-icon" aria-hidden="true">{income ? "＋" : "−"}</span>
      <span className="lr-label">
        <span className="desc">{item.suggestedDescription || item.description}</span>
        <span className="meta">{shortDate(item.date)}, {item.accountName}</span>
        {item.suggestedDescription && <span className="meta">No extrato: {item.description}</span>}
        {item.suggestedMatch && <span className="meta">Parece o lançamento que você já fez: {item.suggestedMatch.description}, {shortDate(item.suggestedMatch.date)}.</span>}
      </span>
      <span className={`val num ${income ? "in" : ""}`}>{income ? "+" : "−"}{formatBRL(Math.abs(item.amountCents))}</span>
    </>
  );
  if (!onOpen) return <div className="list-row">{content}</div>;
  return <button type="button" className="list-row" onClick={onOpen}>{content}</button>;
}
