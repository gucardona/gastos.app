import { useState } from "react";
import { api } from "../../api/client";
import type { PaymentMethod } from "../../api/types";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

export function PaymentsSection() {
  const { paymentMethods, canEdit } = useApp();
  const [showArchived, setShowArchived] = useState(false);
  const active = paymentMethods.filter((p) => !p.archived);
  const archived = paymentMethods.filter((p) => p.archived);
  return (
    <div className="settings">
      {active.map((p) => <Row key={p.id} p={p} />)}
      {canEdit && <Add />}
      {archived.length > 0 && (
        <>
          <button className="chip" style={{ justifySelf: "start", marginTop: 12 }} aria-expanded={showArchived} onClick={() => setShowArchived(!showArchived)}>
            {showArchived ? "Ocultar" : "Mostrar"} arquivadas ({archived.length})
          </button>
          {showArchived && archived.map((p) => <Row key={p.id} p={p} />)}
        </>
      )}
    </div>
  );
}

function Row({ p }: { p: PaymentMethod }) {
  const { wallet, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const path = `/wallets/${wallet.id}/payment-methods/${p.id}`;
  const put = (name: string, icon: string, archived = p.archived) => api.put(path, { name, icon, sortOrder: p.sortOrder, archived });

  if (editing) {
    return <Form initial={{ name: p.name, icon: p.icon }} submit="Salvar" error={error} busy={busy} onCancel={() => setEditing(false)}
      onSubmit={async (d) => { const ok = await run(() => put(d.name, d.icon)); if (ok) setEditing(false); }} />;
  }
  return (
    <div className="item" style={p.archived ? { opacity: 0.65 } : undefined}>
      <div className="line" style={{ borderTop: 0, padding: 0 }}><div><span aria-hidden>{p.icon}</span> {p.name}{p.archived && <small>arquivada</small>}</div></div>
      {canEdit && (
        <div className="item-actions">
          {p.archived ? <button className="chip" disabled={busy} onClick={() => void run(() => put(p.name, p.icon, false))}>Restaurar</button> : (
            <>
              <button className="chip" onClick={() => setEditing(true)}>Editar</button>
              {confirm
                ? <button className="chip danger" disabled={busy} onClick={() => void run(() => api.del(path))}>Confirmar exclusão</button>
                : <button className="chip" onClick={() => setConfirm(true)}>Excluir</button>}
            </>
          )}
        </div>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}

function Add() {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [open, setOpen] = useState(false);
  if (!open) return <button className="btn ghost" style={{ justifySelf: "start", marginTop: 12 }} onClick={() => setOpen(true)}>Nova forma de pagamento</button>;
  return <Form initial={{ name: "", icon: "" }} submit="Criar" error={error} busy={busy} onCancel={() => setOpen(false)}
    onSubmit={async (d) => { if (await run(() => api.post(`/wallets/${wallet.id}/payment-methods`, d))) setOpen(false); }} />;
}

function Form({ initial, submit, onSubmit, onCancel, error, busy }: {
  initial: { name: string; icon: string }; submit: string; onSubmit: (d: { name: string; icon: string }) => Promise<void>; onCancel: () => void; error?: string; busy?: boolean;
}) {
  const [d, setD] = useState(initial);
  return (
    <form className="item inline-form" onSubmit={(e) => { e.preventDefault(); void onSubmit(d); }}>
      <label>Nome<input className="text" aria-label="Nome da forma de pagamento" value={d.name} onChange={(e) => setD({ ...d, name: e.target.value })} /></label>
      <label>Ícone (um emoji)<input className="text" aria-label="Ícone da forma de pagamento" value={d.icon} onChange={(e) => setD({ ...d, icon: e.target.value })} /></label>
      {error && <p className="error" role="alert">{error}</p>}
      <div className="item-actions"><button className="btn" disabled={busy}>{submit}</button><button type="button" className="btn ghost" onClick={onCancel}>Cancelar</button></div>
    </form>
  );
}
