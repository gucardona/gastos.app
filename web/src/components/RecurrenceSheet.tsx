import { useState } from "react";
import { api } from "../api/client";
import type { Recurrence } from "../api/types";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { amountProblem, centsToInput, parseCents } from "../lib/money";
import { percentsFromSplits } from "../lib/splits";
import { ListGroup } from "./ListGroup";
import { Sheet } from "./Sheet";
import { resolveSplits, SplitEditor, type Percents } from "./SplitEditor";

/**
 * The body of the PUT: the API wants the whole object, so what the sheet does not show is kept as it is.
 * Split parts go along only for expenses on a wallet that splits (their amounts are computed by the server):
 * `splits` undefined keeps the stored ones, null sends none (the default split), a list replaces them.
 */
export function recurrenceBody(r: Recurrence, over: Partial<Recurrence>, splitting: boolean, splits?: { userId: number; shareBp: number }[] | null) {
  const x = { ...r, ...over };
  const parts = splits === undefined ? x.splits.map((s) => ({ userId: s.userId, shareBp: s.shareBp })) : splits ?? [];
  return {
    kind: x.kind, amountCents: x.amountCents, description: x.description, categoryId: x.categoryId,
    paymentMethodId: x.paymentMethodId, paidByUserId: x.paidByUserId, toUserId: x.toUserId, dayOfMonth: x.dayOfMonth,
    startDate: x.startDate, endDate: x.endDate, enabled: x.enabled,
    ...(splitting && x.kind === "expense" && parts.length ? { splits: parts } : {}),
  };
}

/** Edit or delete one recurrence; closed while `recurrence` is null. */
export function RecurrenceSheet({ recurrence, onClose }: { recurrence: Recurrence | null; onClose: () => void }) {
  return (
    <Sheet open={recurrence !== null} onClose={onClose} label="Editar recorrência">
      {recurrence && <RecurrenceForm key={recurrence.id} r={recurrence} onClose={onClose} />}
    </Sheet>
  );
}

function RecurrenceForm({ r, onClose }: { r: Recurrence; onClose: () => void }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [amount, setAmount] = useState(centsToInput(r.amountCents));
  const [description, setDescription] = useState(r.description);
  const [day, setDay] = useState(String(r.dayOfMonth));
  const [endDate, setEndDate] = useState(r.endDate ?? "");
  const [percents, setPercents] = useState<Percents | null>(r.splits.length ? percentsFromSplits(r.splits) : null);
  const [problem, setProblem] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const splitting = wallet.splittingEnabled && r.kind === "expense";
  const path = `/wallets/${wallet.id}/recurrences/${r.id}`;

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    const d = Number(day);
    if (!cents) return setProblem(amountProblem(amount));
    if (!Number.isInteger(d) || d < 1 || d > 31) return setProblem("O dia deve estar entre 1 e 31.");
    const { splits, error: splitsError } = resolveSplits(percents);
    if (splitsError) return setProblem(splitsError);
    setProblem("");
    const body = recurrenceBody(r, { amountCents: cents, description: description.trim(), dayOfMonth: d, endDate: endDate || null }, wallet.splittingEnabled, splits ?? null);
    if (await run(() => api.put(path, body), "Recorrência salva")) onClose();
  }

  return (
    <form onSubmit={save}>
      <div className="sheet-head">
        <h2 className="sheet-title">Editar recorrência</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <ListGroup>
        <div className="list-row">
          <label htmlFor="rec-amount" className="lr-label">Valor</label>
          <input id="rec-amount" className="desc-input" inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} />
        </div>
        <div className="list-row">
          <label htmlFor="rec-desc" className="lr-label">Descrição</label>
          <input id="rec-desc" className="desc-input" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div className="list-row">
          <label htmlFor="rec-day" className="lr-label">Dia do mês</label>
          <input id="rec-day" className="desc-input" inputMode="numeric" value={day} onChange={(e) => setDay(e.target.value)} />
        </div>
        <div className="list-row">
          <label htmlFor="rec-end" className="lr-label">Data final</label>
          <input id="rec-end" className="date-input" type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
        </div>
      </ListGroup>
      {splitting && <SplitEditor value={percents} onChange={setPercents} paidBy={r.paidByUserId} cents={parseCents(amount)} />}
      {(problem || error) && <p className="error" role="alert">{problem || error}</p>}
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>Salvar</button>
        {confirmDelete
          ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={(e) => e.detail < 2 && void run(() => api.del(path), "Recorrência excluída").then((ok) => ok && onClose())}>Confirmar exclusão</button>
          : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirmDelete(true)}>Excluir recorrência</button>}
      </div>
    </form>
  );
}
