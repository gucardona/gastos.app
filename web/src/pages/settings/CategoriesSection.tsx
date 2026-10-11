import { useState } from "react";
import { api } from "../../api/client";
import type { Category, Kind } from "../../api/types";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

const ESSENTIALITY = { essential: "Essencial", nonessential: "Não essencial", investment: "Investimento" } as const;

type Draft = { name: string; icon: string; color: string; essentiality: string };
const emptyDraft: Draft = { name: "", icon: "", color: "#0f6579", essentiality: "essential" };

export function CategoriesSection() {
  const { categories, canEdit } = useApp();
  const [kind, setKind] = useState<Kind>("expense");
  const [showArchived, setShowArchived] = useState(false);
  const mine = categories.filter((c) => c.kind === kind);
  const active = mine.filter((c) => !c.archived);
  const archived = mine.filter((c) => c.archived);

  return (
    <div className="settings">
      <div className="seg" role="group" aria-label="Tipo de categoria" style={{ justifySelf: "start", marginBottom: 8 }}>
        <button type="button" aria-pressed={kind === "expense"} onClick={() => setKind("expense")}>Gastos</button>
        <button type="button" aria-pressed={kind === "income"} onClick={() => setKind("income")}>Entradas</button>
      </div>
      {active.map((c) => <Row key={c.id} c={c} />)}
      {canEdit && <NewCategory kind={kind} />}
      {archived.length > 0 && (
        <>
          <button className="chip" style={{ justifySelf: "start", marginTop: 12 }} aria-expanded={showArchived} onClick={() => setShowArchived(!showArchived)}>
            {showArchived ? "Ocultar" : "Mostrar"} arquivadas ({archived.length})
          </button>
          {showArchived && archived.map((c) => <Row key={c.id} c={c} />)}
        </>
      )}
    </div>
  );
}

function body(c: Category, d: Draft, archived = c.archived) {
  return { kind: c.kind, name: d.name, icon: d.icon, color: d.color, essentiality: c.kind === "expense" ? d.essentiality : null, sortOrder: c.sortOrder, archived };
}

function Row({ c }: { c: Category }) {
  const { wallet, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [note, setNote] = useState("");
  const path = `/wallets/${wallet.id}/categories/${c.id}`;
  const draft: Draft = { name: c.name, icon: c.icon, color: c.color, essentiality: c.essentiality ?? "essential" };

  if (editing) {
    return <Form initial={draft} kind={c.kind} submit="Salvar" error={error} busy={busy} onCancel={() => setEditing(false)}
      onSubmit={async (d) => { const ok = await run(() => api.put(path, body(c, d))); if (ok) setEditing(false); return ok; }} />;
  }
  return (
    <div className="item" style={c.archived ? { opacity: 0.65 } : undefined}>
      <div className="line" style={{ borderTop: 0, padding: 0 }}>
        <div><span aria-hidden>{c.icon}</span> {c.name}<small>{c.essentiality ? ESSENTIALITY[c.essentiality] : "Entrada"}{c.archived && ", arquivada"}</small></div>
        <span className="swatch" style={{ background: c.color }} aria-label={`Cor ${c.color}`} />
      </div>
      {canEdit && (
        <div className="item-actions">
          {c.archived ? (
            <button className="chip" disabled={busy} onClick={() => void run(() => api.put(path, body(c, draft, false)))}>Restaurar</button>
          ) : (
            <>
              <button className="chip" onClick={() => setEditing(true)}>Editar</button>
              {confirm
                ? <button className="chip danger" disabled={busy} onClick={async () => {
                    let archivedNow = false;
                    const ok = await run(async () => { archivedNow = (await api.del(path)) !== undefined; });
                    if (ok) setNote(archivedNow ? "Arquivada, porque já tem lançamentos." : "");
                  }}>Confirmar exclusão</button>
                : <button className="chip" onClick={() => setConfirm(true)}>Excluir</button>}
            </>
          )}
        </div>
      )}
      {note && <p className="muted">{note}</p>}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}

function NewCategory({ kind }: { kind: Kind }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [open, setOpen] = useState(false);
  if (!open) return <button className="btn ghost" style={{ justifySelf: "start", marginTop: 12 }} onClick={() => setOpen(true)}>Nova categoria</button>;
  return <Form key={kind} initial={emptyDraft} kind={kind} submit="Criar categoria" error={error} busy={busy} onCancel={() => setOpen(false)}
    onSubmit={async (d) => {
      const ok = await run(() => api.post(`/wallets/${wallet.id}/categories`, { kind, name: d.name, icon: d.icon, color: d.color, ...(kind === "expense" ? { essentiality: d.essentiality } : {}) }));
      if (ok) setOpen(false);
      return ok;
    }} />;
}

function Form({ initial, kind, submit, onSubmit, onCancel, error, busy }: {
  initial: Draft; kind: Kind; submit: string; onSubmit: (d: Draft) => Promise<boolean>; onCancel: () => void; error?: string; busy?: boolean;
}) {
  const [d, setD] = useState(initial);
  const set = (k: keyof Draft) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => setD({ ...d, [k]: e.target.value });
  return (
    <form className="item inline-form" onSubmit={(e) => { e.preventDefault(); void onSubmit(d); }}>
      <label>Nome<input className="text" aria-label="Nome da categoria" value={d.name} onChange={set("name")} /></label>
      <label>Ícone (um emoji)<input className="text" aria-label="Ícone da categoria" value={d.icon} onChange={set("icon")} /></label>
      <label>Cor<input type="color" aria-label="Cor da categoria" value={d.color} onChange={set("color")} /></label>
      {kind === "expense" && (
        <label>Tipo de gasto
          <select aria-label="Tipo de gasto" value={d.essentiality} onChange={set("essentiality")}>
            {Object.entries(ESSENTIALITY).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </label>
      )}
      {error && <p className="error" role="alert">{error}</p>}
      <div className="item-actions">
        <button className="btn" disabled={busy}>{submit}</button>
        <button type="button" className="btn ghost" onClick={onCancel}>Cancelar</button>
      </div>
    </form>
  );
}
