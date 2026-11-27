import { useState } from "react";
import { api } from "../../api/client";
import type { PaymentMethod } from "../../api/types";
import { useApp } from "../../app/context";
import { useAction } from "../../hooks/useAction";
import { CardFields, cardBody, cardDraft } from "./CardFields";
import { EmojiPicker } from "../../ui/EmojiPicker";
import { ListGroup, ListRow } from "../../ui/ListGroup";
import { Sheet } from "../../ui/Sheet";
import { Switch } from "../../ui/Switch";

/**
 * Create (`"new"`) or edit a payment method; closed while `method` is null. A credit card (`card`, or a method that is
 * one) is the same thing plus its card fields, and stays a card: there is no way to turn a plain method into one here.
 */
export function PaymentSheet({ method, card = false, onClose }: { method: PaymentMethod | "new" | null; card?: boolean; onClose: () => void }) {
  const isCard = card || (method !== null && method !== "new" && method.card !== null);
  return (
    <Sheet open={method !== null} onClose={onClose} label={titleOf(method === "new", isCard)}>
      {method && <PaymentForm key={method === "new" ? "new" : method.id} method={method} isCard={isCard} onClose={onClose} />}
    </Sheet>
  );
}

const titleOf = (isNew: boolean, isCard: boolean) => (isCard ? (isNew ? "Novo cartão" : "Editar cartão") : isNew ? "Nova forma de pagamento" : "Editar forma de pagamento");

function PaymentForm({ method, isCard, onClose }: { method: PaymentMethod | "new"; isCard: boolean; onClose: () => void }) {
  const { wallet, user, members } = useApp();
  const { run, busy, error } = useAction();
  const p = method === "new" ? null : method;
  const [name, setName] = useState(p?.name ?? "");
  const [icon, setIcon] = useState(p?.icon ?? "💳");
  const [card, setCard] = useState(() => cardDraft(p?.card ?? null));
  const [confirm, setConfirm] = useState(false);
  const path = p ? `/wallets/${wallet.id}/payment-methods/${p.id}` : "";
  // card goes only on save or create of a card: archive and restore leave it out, and the server keeps it
  const withCard = isCard ? { card: cardBody(card) } : {};
  const put = (archived: boolean, fields = {}) => api.put(path, { name, icon, sortOrder: p?.sortOrder ?? 0, archived, ...fields });
  const done = (ok: boolean) => ok && onClose();
  const noun = isCard ? "Cartão" : "Forma de pagamento";
  const ending = isCard ? "o" : "a"; // "Cartão salvo", "Forma de pagamento salva"
  const title = titleOf(!p, isCard);
  // only for me: each member chooses the method their new transactions start with
  const isDefault = p !== null && members.find((m) => m.userId === user.id)?.defaultPaymentMethodId === p.id;

  return (
    <form onSubmit={async (e) => { e.preventDefault(); done(await run(() => (p ? put(p.archived, withCard) : api.post(`/wallets/${wallet.id}/payment-methods`, { name, icon, ...withCard })), `${noun} ${p ? "salv" : "criad"}${ending}`)); }}>
      <div className="sheet-head">
        <h2 className="sheet-title">{title}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <div className="preview">
        <span className="preview-icon" aria-hidden="true">{icon}</span>
        <strong>{name || (isCard ? "Novo cartão" : "Nova forma de pagamento")}</strong>
      </div>
      <div className="field-row">
        <label htmlFor="pm-name" className="lr-label">Nome</label>
        <input id="pm-name" className="desc-input" aria-label={isCard ? "Nome do cartão" : "Nome da forma de pagamento"} placeholder={isCard ? "Ex.: Nubank" : undefined} value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      {isCard && <CardFields value={card} onChange={setCard} />}
      {p && !p.archived && (
        <ListGroup>
          <ListRow label="Padrão ao lançar" trailing={<Switch checked={isDefault} disabled={busy} label="Padrão ao lançar" onChange={(on) => void run(() => api.put(`/wallets/${wallet.id}/default-payment-method`, { paymentMethodId: on ? p.id : null }), on ? `Seus lançamentos começam com ${p.name}` : "Seus lançamentos começam com a primeira forma de pagamento")} />} />
        </ListGroup>
      )}
      <h3 className="sheet-sub">Ícone</h3>
      <EmojiPicker value={icon} onPick={setIcon} />
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>{p ? "Salvar" : "Criar"}</button>
        {p?.archived && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void run(() => put(false), `${noun} restaurad${ending}`).then(done)}>Restaurar</button>}
        {p && !p.archived && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void run(() => put(true), `${noun} arquivad${ending}`).then(done)}>Arquivar</button>}
        {p && !p.archived && (confirm
          ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={(e) => e.detail < 2 && void run(() => api.del(path), `${noun} excluíd${ending}`).then(done)}>Confirmar exclusão</button>
          : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirm(true)}>{isCard ? "Excluir cartão" : "Excluir forma de pagamento"}</button>)}
      </div>
    </form>
  );
}
