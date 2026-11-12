import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";
import type { Settlement, Transfer } from "../api/types";
import { firstName, useApp } from "../context";
import { useAction, useUndo } from "../hooks/useAction";
import { today } from "../lib/dates";
import { amountProblem, centsToInput, parseCents, typedAmount } from "../lib/money";
import { ListGroup, ListRow } from "./ListGroup";
import { OptionList, PickerView } from "./pickers";
import { Sheet } from "./Sheet";
import { useToast } from "./Toast";

/** Register (`"new"`, or a suggested `Transfer` to check and confirm) or edit a settlement between two people; closed while `settlement` is null. */
export function SettlementSheet({ settlement, onClose }: { settlement: Settlement | Transfer | "new" | null; onClose: () => void }) {
  const saved = settlement && settlement !== "new" && "id" in settlement ? settlement : undefined;
  return (
    <Sheet open={settlement !== null} onClose={onClose} label={saved ? "Editar acerto" : "Registrar acerto"}>
      {settlement && <SettlementForm key={saved?.id ?? "new"} settlement={saved} draft={saved || settlement === "new" ? undefined : settlement} onClose={onClose} />}
    </Sheet>
  );
}

type Picking = "from" | "to" | null;

function SettlementForm({ settlement, draft, onClose }: { settlement?: Settlement; draft?: Transfer; onClose: () => void }) {
  const { wallet, members, user } = useApp();
  const { run, busy, error } = useAction();
  const toast = useToast();
  const undo = useUndo();
  const other = members.find((m) => m.userId !== user.id)?.userId ?? user.id;
  const start = settlement ?? draft;
  const [from, setFrom] = useState(start?.fromUserId ?? other);
  const [to, setTo] = useState(start?.toUserId ?? user.id);
  const [amount, setAmount] = useState(start ? centsToInput(start.amountCents) : "");
  const [date, setDate] = useState(settlement?.date ?? today());
  const [note, setNote] = useState(settlement?.note ?? "");
  const [problem, setProblem] = useState("");
  const [picking, setPicking] = useState<Picking>(null);
  const [confirm, setConfirm] = useState(false);
  const openedFrom = useRef<Picking>(null);
  const path = `/wallets/${wallet.id}/settlements`;

  const name = (id: number) => {
    const m = members.find((x) => x.userId === id);
    return m ? (id === user.id ? `${firstName(m.name)} (você)` : firstName(m.name)) : "Alguém";
  };
  const options = members.map((m) => ({ value: m.userId, label: name(m.userId) }));

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    if (!cents) return setProblem(amountProblem(amount));
    if (from === to) return setProblem("Escolha pessoas diferentes para quem paga e quem recebe.");
    setProblem("");
    const body = { fromUserId: from, toUserId: to, amountCents: cents, date, note: note.trim() };
    if (await run(() => (settlement ? api.put(`${path}/${settlement.id}`, body) : api.post(path, body)), settlement ? "Acerto salvo" : "Acerto registrado")) onClose();
  }

  async function remove(s: Settlement) {
    if (!(await run(() => api.del(`${path}/${s.id}`)))) return;
    const again = { fromUserId: s.fromUserId, toUserId: s.toUserId, amountCents: s.amountCents, date: s.date, note: s.note };
    // the copy is a plain settlement: a deleted occurrence of a recurrence is never made again by the server
    toast("Acerto excluído", { action: undo(() => api.post(path, again)) });
    onClose();
  }

  // coming back from a picker, focus goes to the row that opened it
  useEffect(() => {
    if (picking === null && openedFrom.current) {
      document.getElementById(`row-${openedFrom.current}`)?.focus();
      openedFrom.current = null;
    }
  }, [picking]);
  const choose = (p: Exclude<Picking, null>) => {
    openedFrom.current = p;
    setPicking(p);
  };

  if (picking) {
    return (
      <PickerView title={picking === "from" ? "Quem pagou" : "Quem recebeu"} onBack={() => setPicking(null)}>
        <OptionList options={options} value={picking === "from" ? from : to} onPick={(id) => { (picking === "from" ? setFrom : setTo)(id); setPicking(null); }} />
      </PickerView>
    );
  }

  return (
    <form onSubmit={save}>
      <div className="sheet-head">
        <h2 className="sheet-title">{settlement ? "Editar acerto" : "Registrar acerto"}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <ListGroup>
        <ListRow id="row-from" label="Quem pagou" value={name(from)} onClick={() => choose("from")} chevron />
        <ListRow id="row-to" label="Quem recebeu" value={name(to)} onClick={() => choose("to")} chevron />
        <div className="list-row">
          <label htmlFor="st-amount" className="lr-label">Valor</label>
          <input id="st-amount" className="desc-input" inputMode="numeric" aria-label="Valor do acerto" value={amount} onChange={(e) => setAmount(typedAmount(e.target.value))} />
        </div>
        <div className="list-row">
          <label htmlFor="st-date" className="lr-label">Data</label>
          <input id="st-date" className="date-input" type="date" aria-label="Data do acerto" value={date} onChange={(e) => setDate(e.target.value)} />
        </div>
        <div className="list-row">
          <label htmlFor="st-note" className="lr-label">Observação</label>
          <input id="st-note" className="desc-input" placeholder="opcional" aria-label="Observação do acerto" value={note} onChange={(e) => setNote(e.target.value)} />
        </div>
      </ListGroup>
      {(problem || error) && <p className="error" role="alert">{problem || error}</p>}
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>{settlement ? "Salvar" : "Registrar"}</button>
        {settlement && (confirm
          ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={(e) => e.detail < 2 && void remove(settlement)}>Confirmar exclusão</button>
          : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirm(true)}>Excluir acerto</button>)}
      </div>
    </form>
  );
}
