import { useState } from "react";
import { api } from "../api/client";
import { useRecurrences } from "../api/queries";
import type { Recurrence } from "../api/types";
import { PageBar } from "../components/PageBar";
import { Switch } from "../components/Switch";
import { firstName, useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { formatBRL, parseCents } from "../lib/money";

const longDate = (iso: string) => new Date(`${iso}T12:00:00`).toLocaleDateString("pt-BR", { day: "numeric", month: "long" });
const centsToInput = (c: number) => (c / 100).toFixed(2).replace(".", ",");

export function Recurring() {
  const { wallet } = useApp();
  const list = useRecurrences(wallet.id);

  return (
    <>
      <PageBar />
      <h1 style={{ font: "700 28px var(--display)", margin: "8px 0 4px" }}>Recorrentes</h1>
      <p className="muted" style={{ margin: "0 0 12px" }}>Lançamentos que se repetem todo mês. Eles aparecem na lista quando a data chega.</p>
      {list.error ? (
        <div className="empty"><p>Não foi possível carregar as recorrências.</p><button className="btn" onClick={() => void list.refetch()}>Tentar de novo</button></div>
      ) : !list.data ? (
        <><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></>
      ) : list.data.length === 0 ? (
        <div className="empty"><p>Nenhuma recorrência ainda. Ao lançar um gasto ou uma entrada, ative "Repetir todo mês".</p></div>
      ) : (
        list.data.map((r) => <Item key={r.id} r={r} />)
      )}
    </>
  );
}

function Item({ r }: { r: Recurrence }) {
  const { wallet, categories, paymentMethods, members, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const [editing, setEditing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const cat = categories.find((c) => c.id === r.categoryId);
  const name = (id: number | null) => firstName(members.find((m) => m.userId === id)?.name ?? "Alguém");
  const title = r.description || (r.kind === "settlement" ? `Acerto de ${name(r.paidByUserId)} para ${name(r.toUserId)}` : cat?.name ?? "Recorrência");
  const sign = r.kind === "income" ? "+" : "−";

  // the API expects the whole object on PUT; the rest of the form is preserved as is
  const body = (over: Partial<Recurrence> = {}) => {
    const x = { ...r, ...over };
    return {
      kind: x.kind, amountCents: x.amountCents, description: x.description, categoryId: x.categoryId,
      paymentMethodId: x.paymentMethodId, paidByUserId: x.paidByUserId, toUserId: x.toUserId, dayOfMonth: x.dayOfMonth,
      startDate: x.startDate, endDate: x.endDate, enabled: x.enabled,
      ...(wallet.splittingEnabled && x.kind === "expense" && x.splits.length ? { splits: x.splits.map((s) => ({ userId: s.userId, shareBp: s.shareBp })) } : {}),
    };
  };

  if (editing) return <EditForm r={r} body={body} onDone={() => setEditing(false)} />;

  return (
    <div className="item" style={r.enabled ? undefined : { opacity: 0.6 }}>
      <div className="row" style={{ borderTop: 0, minHeight: 0, padding: 0 }}>
        <span className="emoji" aria-hidden>{r.kind === "settlement" ? "⇄" : cat?.icon}</span>
        <span>
          <span className="desc">{title}</span>
          <span className="meta" style={{ display: "block" }}>
            {[cat?.name, paymentMethods.find((p) => p.id === r.paymentMethodId)?.name].filter(Boolean).join(", ")}
            {cat && <br />}
            Todo dia {r.dayOfMonth}
            {r.enabled && r.nextDate ? `, próxima em ${longDate(r.nextDate)}` : r.enabled ? ", encerrada" : ", pausada"}
            {r.endDate && r.enabled && r.nextDate ? `, até ${longDate(r.endDate)}` : ""}
          </span>
        </span>
        <span className={`val num ${r.kind === "income" ? "in" : ""}`}>{sign}{formatBRL(r.amountCents)}</span>
      </div>
      {canEdit && (
        <div className="item-actions">
          <label className="inline">
            <Switch checked={r.enabled} disabled={busy} label={`${r.enabled ? "Pausar" : "Retomar"} ${title}`} onChange={(v) => void run(() => api.put(`/wallets/${wallet.id}/recurrences/${r.id}`, body({ enabled: v })))} />
            <span className="muted">{r.enabled ? "Ativa" : "Pausada"}</span>
          </label>
          <button className="chip" onClick={() => setEditing(true)}>Editar</button>
          {confirmDelete ? (
            <button className="chip danger" disabled={busy} onClick={() => void run(() => api.del(`/wallets/${wallet.id}/recurrences/${r.id}`))}>Confirmar exclusão</button>
          ) : (
            <button className="chip" onClick={() => setConfirmDelete(true)}>Excluir</button>
          )}
        </div>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}

function EditForm({ r, body, onDone }: { r: Recurrence; body: (over?: Partial<Recurrence>) => object; onDone: () => void }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [amount, setAmount] = useState(centsToInput(r.amountCents));
  const [description, setDescription] = useState(r.description);
  const [day, setDay] = useState(String(r.dayOfMonth));
  const [endDate, setEndDate] = useState(r.endDate ?? "");
  const [problem, setProblem] = useState("");

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    const d = Number(day);
    if (!cents) return setProblem("Informe um valor maior que zero.");
    if (!Number.isInteger(d) || d < 1 || d > 31) return setProblem("O dia deve estar entre 1 e 31.");
    setProblem("");
    const ok = await run(() => api.put(`/wallets/${wallet.id}/recurrences/${r.id}`, body({ amountCents: cents, description: description.trim(), dayOfMonth: d, endDate: endDate || null })));
    if (ok) onDone();
  }

  return (
    <form className="item inline-form" onSubmit={save}>
      <label>Valor<input className="text" inputMode="decimal" aria-label="Valor" value={amount} onChange={(e) => setAmount(e.target.value)} /></label>
      <label>Descrição<input className="text" aria-label="Descrição" value={description} onChange={(e) => setDescription(e.target.value)} /></label>
      <label>Dia do mês<input className="text" inputMode="numeric" aria-label="Dia do mês" value={day} onChange={(e) => setDay(e.target.value)} /></label>
      <label>Até (opcional)<input className="text" type="date" aria-label="Data final" value={endDate} onChange={(e) => setEndDate(e.target.value)} /></label>
      {(problem || error) && <p className="error" role="alert">{problem || error}</p>}
      <div className="item-actions">
        <button className="btn" disabled={busy}>Salvar</button>
        <button type="button" className="btn ghost" onClick={onDone}>Cancelar</button>
      </div>
    </form>
  );
}
