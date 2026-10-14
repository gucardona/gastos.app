import { useState } from "react";
import { api } from "../api/client";
import type { Settlement } from "../api/types";
import { firstName, useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { today } from "../lib/dates";
import { centsToInput, parseCents } from "../lib/money";
import { ListGroup, ListRow } from "./ListGroup";
import { OptionList, PickerView } from "./pickers";
import { Sheet } from "./Sheet";

/** Register (`"new"`) or edit a settlement between two people; closed while `settlement` is null. */
export function SettlementSheet({ settlement, onClose }: { settlement: Settlement | "new" | null; onClose: () => void }) {
  return (
    <Sheet open={settlement !== null} onClose={onClose} label={settlement === "new" ? "Registrar acerto" : "Editar acerto"}>
      {settlement && <SettlementForm key={settlement === "new" ? "new" : settlement.id} settlement={settlement === "new" ? undefined : settlement} onClose={onClose} />}
    </Sheet>
  );
}

type Picking = "from" | "to" | null;

function SettlementForm({ settlement, onClose }: { settlement?: Settlement; onClose: () => void }) {
  const { wallet, members, user } = useApp();
  const { run, busy, error } = useAction();
  const other = members.find((m) => m.userId !== user.id)?.userId ?? user.id;
  const [from, setFrom] = useState(settlement?.fromUserId ?? other);
  const [to, setTo] = useState(settlement?.toUserId ?? user.id);
  const [amount, setAmount] = useState(settlement ? centsToInput(settlement.amountCents) : "");
  const [date, setDate] = useState(settlement?.date ?? today());
  const [note, setNote] = useState(settlement?.note ?? "");
  const [problem, setProblem] = useState("");
  const [picking, setPicking] = useState<Picking>(null);
  const [confirm, setConfirm] = useState(false);
  const path = `/wallets/${wallet.id}/settlements`;

  const name = (id: number) => {
    const m = members.find((x) => x.userId === id);
    return m ? (id === user.id ? `${firstName(m.name)} (você)` : firstName(m.name)) : "Alguém";
  };
  const options = members.map((m) => ({ value: m.userId, label: name(m.userId) }));

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    if (!cents) return setProblem("Informe um valor maior que zero.");
    if (from === to) return setProblem("Escolha pessoas diferentes para quem paga e quem recebe.");
    setProblem("");
    const body = { fromUserId: from, toUserId: to, amountCents: cents, date, note: note.trim() };
    if (await run(() => (settlement ? api.put(`${path}/${settlement.id}`, body) : api.post(path, body)))) onClose();
  }

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
        <ListRow label="Quem pagou" value={name(from)} onClick={() => setPicking("from")} chevron />
        <ListRow label="Quem recebeu" value={name(to)} onClick={() => setPicking("to")} chevron />
        <div className="list-row">
          <label htmlFor="st-amount" className="lr-label">Valor</label>
          <input id="st-amount" className="desc-input" inputMode="decimal" aria-label="Valor do acerto" value={amount} onChange={(e) => setAmount(e.target.value)} />
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
          ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={() => void run(() => api.del(`${path}/${settlement.id}`)).then((ok) => ok && onClose())}>Confirmar exclusão</button>
          : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirm(true)}>Excluir acerto</button>)}
      </div>
    </form>
  );
}
