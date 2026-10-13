import { useState } from "react";
import { api } from "../api/client";
import { useRecurrences } from "../api/queries";
import type { Recurrence } from "../api/types";
import { firstName, useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { formatBRL } from "../lib/money";
import { ListGroup } from "./ListGroup";
import { RecurrenceSheet, recurrenceBody } from "./RecurrenceSheet";
import { Switch } from "./Switch";

const longDate = (iso: string) => new Date(`${iso}T12:00:00`).toLocaleDateString("pt-BR", { day: "numeric", month: "long" });

/** Everything that repeats every month: pause it with the switch, tap the row to edit. */
export function RecurringSection() {
  const { wallet, canEdit } = useApp();
  const list = useRecurrences(wallet.id);
  const [editing, setEditing] = useState<Recurrence | null>(null);

  if (list.error) return <p className="muted sec-note">Não foi possível carregar as recorrências.</p>;
  if (!list.data) return null;
  if (list.data.length === 0 && !canEdit) return null;

  return (
    <section className="sec">
      <h2>Repetem todo mês</h2>
      {list.data.length === 0 ? (
        <p className="sub">Nenhuma recorrência ainda. Ao lançar um gasto ou uma entrada, ative "Repetir todo mês".</p>
      ) : (
        <ListGroup>
          {list.data.map((r) => <Row key={r.id} r={r} onOpen={() => setEditing(r)} />)}
        </ListGroup>
      )}
      <RecurrenceSheet recurrence={editing} onClose={() => setEditing(null)} />
    </section>
  );
}

function Row({ r, onOpen }: { r: Recurrence; onOpen: () => void }) {
  const { wallet, categories, paymentMethods, members, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const cat = categories.find((c) => c.id === r.categoryId);
  const person = (id: number | null) => firstName(members.find((m) => m.userId === id)?.name ?? "Alguém");
  const title = r.description || (r.kind === "settlement" ? `Acerto de ${person(r.paidByUserId)} para ${person(r.toUserId)}` : cat?.name ?? "Recorrência");
  const status = r.enabled ? (r.nextDate ? `, próxima em ${longDate(r.nextDate)}` : ", encerrada") : ", pausada";
  const method = paymentMethods.find((p) => p.id === r.paymentMethodId)?.name;
  const content = (
    <>
      <span className="lr-icon" aria-hidden="true">{r.kind === "settlement" ? "⇄" : cat?.icon}</span>
      <span className="lr-label">
        <span className="desc">{title}</span>
        <span className="meta">{[cat?.name, method].filter(Boolean).join(", ")}</span>
        <span className="meta">Todo dia {r.dayOfMonth}{status}</span>
      </span>
      <span className={`val num ${r.kind === "income" ? "in" : ""}`}>{r.kind === "income" ? "+" : "−"}{formatBRL(r.amountCents)}</span>
    </>
  );

  if (!canEdit) return <div className="list-row rec-row" style={r.enabled ? undefined : { opacity: 0.6 }}>{content}</div>;
  return (
    <>
      <div className="rec-row" style={r.enabled ? undefined : { opacity: 0.6 }}>
        <button type="button" className="list-row rec-open" onClick={onOpen}>{content}</button>
        <Switch checked={r.enabled} disabled={busy} label={`${r.enabled ? "Pausar" : "Retomar"} ${title}`} onChange={(v) => void run(() => api.put(`/wallets/${wallet.id}/recurrences/${r.id}`, recurrenceBody(r, { enabled: v }, wallet.splittingEnabled)))} />
      </div>
      {error && <p className="error" role="alert">{error}</p>}
    </>
  );
}
