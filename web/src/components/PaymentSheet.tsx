import { useState } from "react";
import { api } from "../api/client";
import type { PaymentMethod } from "../api/types";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { EmojiPicker } from "./EmojiPicker";
import { Sheet } from "./Sheet";

/** Create (`"new"`) or edit a payment method; closed while `method` is null. */
export function PaymentSheet({ method, onClose }: { method: PaymentMethod | "new" | null; onClose: () => void }) {
  return (
    <Sheet open={method !== null} onClose={onClose} label={method === "new" ? "Nova forma de pagamento" : "Editar forma de pagamento"}>
      {method && <PaymentForm key={method === "new" ? "new" : method.id} method={method} onClose={onClose} />}
    </Sheet>
  );
}

function PaymentForm({ method, onClose }: { method: PaymentMethod | "new"; onClose: () => void }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const p = method === "new" ? null : method;
  const [name, setName] = useState(p?.name ?? "");
  const [icon, setIcon] = useState(p?.icon ?? "💳");
  const [confirm, setConfirm] = useState(false);
  const path = p ? `/wallets/${wallet.id}/payment-methods/${p.id}` : "";
  const put = (archived: boolean) => api.put(path, { name, icon, sortOrder: p?.sortOrder ?? 0, archived });
  const done = (ok: boolean) => ok && onClose();

  return (
    <form onSubmit={async (e) => { e.preventDefault(); done(await run(() => (p ? put(p.archived) : api.post(`/wallets/${wallet.id}/payment-methods`, { name, icon })))); }}>
      <div className="sheet-head">
        <h2 className="sheet-title">{p ? "Editar forma de pagamento" : "Nova forma de pagamento"}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <div className="preview">
        <span className="preview-icon" aria-hidden="true">{icon}</span>
        <strong>{name || "Nova forma de pagamento"}</strong>
      </div>
      <div className="field-row">
        <label htmlFor="pm-name" className="lr-label">Nome</label>
        <input id="pm-name" className="desc-input" aria-label="Nome da forma de pagamento" value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      <h3 className="sheet-sub">Ícone</h3>
      <EmojiPicker value={icon} onPick={setIcon} />
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>{p ? "Salvar" : "Criar"}</button>
        {p?.archived && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void run(() => put(false)).then(done)}>Restaurar</button>}
        {p && !p.archived && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void run(() => put(true)).then(done)}>Arquivar</button>}
        {p && !p.archived && (confirm
          ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={() => void run(() => api.del(path)).then(done)}>Confirmar exclusão</button>
          : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirm(true)}>Excluir forma de pagamento</button>)}
      </div>
    </form>
  );
}
