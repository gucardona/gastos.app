import { useState } from "react";
import { Link } from "react-router";
import { api } from "../api/client";
import { useSettlements, useSummary } from "../api/queries";
import type { Settlement } from "../api/types";
import { PageBar } from "../components/PageBar";
import { firstName, useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { dayLabel, today } from "../lib/dates";
import { formatBRL, parseCents } from "../lib/money";

const centsToInput = (c: number) => (c / 100).toFixed(2).replace(".", ",");

export function Division() {
  const { wallet, month, members, user, canEdit } = useApp();
  const summary = useSummary(wallet.id, month);
  const history = useSettlements(wallet.id);
  const register = useAction();
  const [adding, setAdding] = useState(false);

  const who = (id: number) => (id === user.id ? "Você" : firstName(members.find((m) => m.userId === id)?.name ?? "Alguém"));

  if (!wallet.splittingEnabled) {
    return (
      <>
        <PageBar />
        <Title />
        <div className="empty">
          <p>A divisão de gastos está desligada em {wallet.name}. Ligue para ver quem pagou o quê e acertar as contas entre vocês.</p>
          {wallet.role === "owner" && <Link className="btn" to="/ajustes?s=carteira">Ligar a divisão</Link>}
        </div>
      </>
    );
  }
  if (summary.error || history.error) {
    return <><PageBar /><Title /><div className="empty"><p>Não foi possível carregar a divisão.</p><button type="button" className="btn" onClick={() => { void summary.refetch(); void history.refetch(); }}>Tentar de novo</button></div></>;
  }
  if (!summary.data || !history.data) return <><PageBar /><Title /><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></>;

  const balances = summary.data.memberBalances ?? [];
  const suggestions = summary.data.suggestedSettlements ?? [];

  return (
    <>
      <PageBar />
      <Title />

      <section className="sec" style={{ marginTop: 16 }}>
        <h2>Saldo entre vocês</h2>
        <p className="sub">Soma de todos os gastos divididos e acertos já registrados.</p>
        {balances.map((b) => (
          <div className="line" key={b.userId}>
            <div>{who(b.userId)}<small>{b.netCents > 0 ? "tem a receber" : b.netCents < 0 ? "deve" : "está quite"}</small></div>
            <span className="num" style={{ fontWeight: 600, color: b.netCents > 0 ? "var(--income)" : b.netCents < 0 ? "var(--over)" : "var(--muted)" }}>
              {b.netCents === 0 ? formatBRL(0) : `${b.netCents > 0 ? "+" : "−"}${formatBRL(Math.abs(b.netCents))}`}
            </span>
          </div>
        ))}
      </section>

      <section className="sec">
        <h2>Para acertar</h2>
        {suggestions.length === 0 ? (
          <p className="sub">Vocês estão quites.</p>
        ) : (
          <>
            <p className="sub">O menor número de transferências que zera o saldo.</p>
            {suggestions.map((t) => (
              <div className="settle" key={`${t.fromUserId}-${t.toUserId}`} style={{ marginBottom: 12 }}>
                <p><span className="big num">{formatBRL(t.amountCents)}</span></p>
                <p>{who(t.fromUserId)} deve esse valor {t.toUserId === user.id ? "a você" : `para ${who(t.toUserId)}`}.</p>
                {canEdit && (
                  <button type="button" className="btn full" disabled={register.busy}
                    onClick={() => void register.run(() => api.post(`/wallets/${wallet.id}/settlements`, { fromUserId: t.fromUserId, toUserId: t.toUserId, amountCents: t.amountCents, date: today() }))}>
                    Registrar acerto
                  </button>
                )}
              </div>
            ))}
            {register.error && <p className="error" role="alert">{register.error}</p>}
          </>
        )}
        {canEdit && !adding && <button type="button" className="btn ghost" onClick={() => setAdding(true)}>Registrar outro acerto</button>}
        {adding && <SettlementForm submit="Registrar" onDone={() => setAdding(false)} />}
      </section>

      <section className="sec">
        <h2>Histórico de acertos</h2>
        {history.data.length === 0 ? <p className="sub">Nenhum acerto registrado ainda.</p> : <p className="sub">Os últimos meses.</p>}
        {history.data.map((s) => <HistoryItem key={s.id} s={s} />)}
      </section>
    </>
  );
}

const Title = () => <h1 style={{ font: "700 28px var(--display)", margin: "8px 0 4px" }}>Divisão</h1>;

function HistoryItem({ s }: { s: Settlement }) {
  const { wallet, members, user, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const who = (id: number) => (id === user.id ? "Você" : firstName(members.find((m) => m.userId === id)?.name ?? "Alguém"));

  if (editing) return <SettlementForm initial={s} submit="Salvar" onDone={() => setEditing(false)} />;
  return (
    <div className="item">
      <div className="line" style={{ borderTop: 0, padding: 0 }}>
        <div>{who(s.fromUserId)} pagou {s.toUserId === user.id ? "a você" : who(s.toUserId)}<small>{dayLabel(s.date)}{s.note && `. ${s.note}`}{s.recurrenceId && ". Recorrente"}</small></div>
        <span className="num" style={{ fontWeight: 600 }}>{formatBRL(s.amountCents)}</span>
      </div>
      {canEdit && (
        <div className="item-actions">
          <button type="button" className="chip" onClick={() => setEditing(true)}>Editar</button>
          {confirm
            ? <button type="button" className="chip danger" disabled={busy} onClick={() => void run(() => api.del(`/wallets/${wallet.id}/settlements/${s.id}`))}>Confirmar exclusão</button>
            : <button type="button" className="chip" onClick={() => setConfirm(true)}>Excluir</button>}
        </div>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}

function SettlementForm({ initial, submit, onDone }: { initial?: Settlement; submit: string; onDone: () => void }) {
  const { wallet, members, user } = useApp();
  const { run, busy, error } = useAction();
  const other = members.find((m) => m.userId !== user.id)?.userId ?? user.id;
  const [from, setFrom] = useState(initial?.fromUserId ?? other);
  const [to, setTo] = useState(initial?.toUserId ?? user.id);
  const [amount, setAmount] = useState(initial ? centsToInput(initial.amountCents) : "");
  const [date, setDate] = useState(initial?.date ?? today());
  const [note, setNote] = useState(initial?.note ?? "");
  const [problem, setProblem] = useState("");

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    if (!cents) return setProblem("Informe um valor maior que zero.");
    if (from === to) return setProblem("Escolha pessoas diferentes para quem paga e quem recebe.");
    setProblem("");
    const body = { fromUserId: from, toUserId: to, amountCents: cents, date, note: note.trim() };
    const path = `/wallets/${wallet.id}/settlements`;
    if (await run(() => (initial ? api.put(`${path}/${initial.id}`, body) : api.post(path, body)))) onDone();
  }

  const select = (label: string, value: number, set: (n: number) => void) => (
    <label>{label}
      <select aria-label={label} value={value} onChange={(e) => set(Number(e.target.value))}>
        {members.map((m) => <option key={m.userId} value={m.userId}>{m.userId === user.id ? `${firstName(m.name)} (você)` : firstName(m.name)}</option>)}
      </select>
    </label>
  );

  return (
    <form className="item inline-form" onSubmit={save}>
      {select("Quem pagou", from, setFrom)}
      {select("Quem recebeu", to, setTo)}
      <label>Valor<input className="text" inputMode="decimal" aria-label="Valor do acerto" value={amount} onChange={(e) => setAmount(e.target.value)} /></label>
      <label>Data<input className="text" type="date" aria-label="Data do acerto" value={date} onChange={(e) => setDate(e.target.value)} /></label>
      <label>Observação (opcional)<input className="text" aria-label="Observação do acerto" value={note} onChange={(e) => setNote(e.target.value)} /></label>
      {(problem || error) && <p className="error" role="alert">{problem || error}</p>}
      <div className="item-actions">
        <button type="submit" className="btn" disabled={busy}>{submit}</button>
        <button type="button" className="btn ghost" onClick={onDone}>Cancelar</button>
      </div>
    </form>
  );
}
