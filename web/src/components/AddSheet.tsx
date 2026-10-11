import { useEffect, useRef, useState } from "react";
import { ApiError, api } from "../api/client";
import { useRefresh } from "../api/queries";
import type { Kind, Transaction } from "../api/types";
import { useApp } from "../context";
import { toISODate, today } from "../lib/dates";
import { formatBRL, parseCents } from "../lib/money";
import { equalPercents, sharesFromPercents } from "../lib/splits";

/** Quick-add sheet, built on a native <dialog> (focus trap and Esc for free). */
export function AddSheet({ state, onClose }: { state: { tx?: Transaction } | null; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const open = state !== null;

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className="sheet"
      aria-label={state?.tx ? "Editar lançamento" : "Novo lançamento"}
      onClose={onClose}
      onClick={(e) => e.target === ref.current && onClose()}
    >
      {open && <AddForm key={state?.tx?.id ?? "new"} tx={state?.tx} onClose={onClose} />}
    </dialog>
  );
}

const centsToInput = (c: number) => (c / 100).toFixed(2).replace(".", ",");
const yesterday = () => {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  return toISODate(d);
};

function AddForm({ tx, onClose }: { tx?: Transaction; onClose: () => void }) {
  const { user, wallet, categories, paymentMethods, members } = useApp();
  const refresh = useRefresh();

  const [kind, setKind] = useState<Kind>(tx?.kind ?? "expense");
  const [amount, setAmount] = useState(tx ? centsToInput(tx.amountCents) : "");
  const [description, setDescription] = useState(tx?.description ?? "");
  const [categoryId, setCategoryId] = useState<number | null>(tx?.categoryId ?? null);
  const [methodId, setMethodId] = useState<number | null>(tx?.paymentMethodId ?? null);
  const [date, setDate] = useState(tx?.date ?? today());
  const [otherDay, setOtherDay] = useState(false);
  const [paidBy, setPaidBy] = useState(tx?.paidByUserId ?? user.id);
  const [percents, setPercents] = useState<Record<number, string> | null>(
    tx && tx.splits.length ? Object.fromEntries(tx.splits.map((s) => [s.userId, String(s.shareBp / 100).replace(".", ",")])) : null,
  );
  const [repeat, setRepeat] = useState(false);
  const [showAllCats, setShowAllCats] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const amountRef = useRef<HTMLInputElement>(null);

  const cats = categories.filter((c) => c.kind === kind && (!c.archived || c.id === tx?.categoryId));
  const category = cats.find((c) => c.id === categoryId) ?? cats[0];
  const methods = paymentMethods.filter((p) => !p.archived || p.id === tx?.paymentMethodId);
  const method = methods.find((p) => p.id === methodId) ?? (tx ? undefined : methods[0]);
  const shownCats = showAllCats ? cats : cats.slice(0, 8);

  const splitting = wallet.splittingEnabled && kind === "expense" && members.length > 1;
  const defaultIds = members.filter((m) => m.splitsByDefault).map((m) => m.userId);
  const nameOf = (id: number) => members.find((m) => m.userId === id)?.name.split(" ")[0] ?? "";
  const cents = parseCents(amount);

  async function save(again: boolean) {
    const errs: Record<string, string> = {};
    if (!cents) errs.amountCents = "Informe um valor maior que zero.";
    if (!description.trim()) errs.description = "Descreva o lançamento.";
    if (!category) errs.categoryId = "Escolha uma categoria.";
    let splits: { userId: number; shareBp: number }[] | undefined;
    if (splitting && percents) {
      splits = sharesFromPercents(percents) ?? undefined;
      if (!splits) errs.splits = "As partes precisam somar 100%.";
    }
    setErrors(errs);
    setGeneral("");
    if (Object.keys(errs).length > 0 || !cents || !category) return;

    const base = {
      kind, amountCents: cents, description: description.trim(), categoryId: category.id,
      paymentMethodId: method?.id ?? null, paidByUserId: paidBy, ...(splitting && splits ? { splits } : {}),
    };
    setBusy(true);
    try {
      const wp = `/wallets/${wallet.id}`;
      if (tx) await api.put(`${wp}/transactions/${tx.id}`, { ...base, date });
      else if (repeat) await api.post(`${wp}/recurrences`, { ...base, dayOfMonth: Number(date.slice(8)), startDate: date });
      else await api.post(`${wp}/transactions`, { ...base, date });
      refresh(wallet.id);
      if (again) {
        setAmount("");
        setDescription("");
        setRepeat(false);
        amountRef.current?.focus();
      } else onClose();
    } catch (e) {
      if (e instanceof ApiError) {
        setErrors(e.fields);
        setGeneral(Object.keys(e.fields).length ? "" : e.message);
      } else setGeneral("Sem conexão. Tente de novo.");
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!tx) return;
    setBusy(true);
    try {
      await api.del(`/wallets/${wallet.id}/transactions/${tx.id}`);
      refresh(wallet.id);
      onClose();
    } catch (e) {
      setGeneral(e instanceof ApiError ? e.message : "Sem conexão. Tente de novo.");
      setBusy(false);
    }
  }

  const err = (k: string) => errors[k] && <p className="error" role="alert">{errors[k]}</p>;

  return (
    <form onSubmit={(e) => { e.preventDefault(); void save(false); }}>
      <div className="grab" />
      <div className="sheet-head">
        <div className="seg" role="group" aria-label="Tipo">
          <button type="button" aria-pressed={kind === "expense"} onClick={() => setKind("expense")}>Gasto</button>
          <button type="button" aria-pressed={kind === "income"} onClick={() => setKind("income")}>Entrada</button>
        </div>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>

      <div className="amount">
        <span className="cur">R$</span>
        <input ref={amountRef} autoFocus inputMode="decimal" placeholder="0,00" aria-label="Valor" value={amount} onChange={(e) => setAmount(e.target.value)} />
      </div>
      {err("amountCents")}

      <div className="field">
        <label htmlFor="desc">Descrição</label>
        <input id="desc" className="text" value={description} onChange={(e) => setDescription(e.target.value)} />
        {err("description")}
      </div>

      <div className="field">
        <span className="lbl">Categoria</span>
        <div className="chips">
          {shownCats.map((c) => (
            <button type="button" className="chip" key={c.id} aria-pressed={c.id === category?.id} onClick={() => setCategoryId(c.id)}>{c.icon} {c.name}</button>
          ))}
          {cats.length > 8 && <button type="button" className="chip" onClick={() => setShowAllCats(!showAllCats)}>{showAllCats ? "Menos" : "Mais"}</button>}
        </div>
        {err("categoryId")}
      </div>

      <div className="field">
        <span className="lbl">Pagamento</span>
        <div className="chips">
          {methods.map((p) => (
            <button type="button" className="chip" key={p.id} aria-pressed={p.id === method?.id} onClick={() => setMethodId(p.id)}>{p.name}</button>
          ))}
        </div>
      </div>

      <div className="field">
        <span className="lbl">Data</span>
        <div className="chips">
          <button type="button" className="chip" aria-pressed={!otherDay && date === today()} onClick={() => { setOtherDay(false); setDate(today()); }}>Hoje</button>
          <button type="button" className="chip" aria-pressed={!otherDay && date === yesterday()} onClick={() => { setOtherDay(false); setDate(yesterday()); }}>Ontem</button>
          <button type="button" className="chip" aria-pressed={otherDay || (date !== today() && date !== yesterday())} onClick={() => setOtherDay(true)}>Outro dia</button>
        </div>
        {(otherDay || (date !== today() && date !== yesterday())) && (
          <input type="date" className="text" aria-label="Data do lançamento" value={date} onChange={(e) => setDate(e.target.value)} />
        )}
        {err("date")}
      </div>

      {splitting && (
        <>
          <div className="field">
            <span className="lbl">Quem pagou</span>
            <div className="chips">
              {members.map((m) => (
                <button type="button" className="chip" key={m.userId} aria-pressed={paidBy === m.userId} onClick={() => setPaidBy(m.userId)}>{m.name.split(" ")[0]}</button>
              ))}
            </div>
          </div>
          <div className="line">
            <div>
              {percents ? "Divisão personalizada" : `Dividir com ${defaultIds.filter((id) => id !== paidBy).map(nameOf).join(", ") || "ninguém"}`}
              <small>
                {percents ? "Ajuste a porcentagem de cada pessoa." : cents ? `Igual entre ${defaultIds.length}, ${formatBRL(Math.floor(cents / Math.max(defaultIds.length, 1)))} para cada` : "Igual entre os participantes"}
              </small>
            </div>
            <button type="button" className="chip" onClick={() => setPercents(percents ? null : equalPercents(defaultIds.length ? defaultIds : members.map((m) => m.userId)))}>
              {percents ? "Igualar" : "Ajustar"}
            </button>
          </div>
          {percents && (
            <div className="split-edit">
              {members.map((m) => (
                <label key={m.userId}>
                  {m.name.split(" ")[0]}
                  <span><input inputMode="decimal" aria-label={`Porcentagem de ${m.name}`} value={percents[m.userId] ?? ""} onChange={(e) => setPercents({ ...percents, [m.userId]: e.target.value })} /> %</span>
                </label>
              ))}
            </div>
          )}
          {err("splits")}
        </>
      )}

      {!tx && (
        <div className="line">
          <div>Repetir todo mês<small>Cria um lançamento recorrente a partir desta data</small></div>
          <button type="button" className="switch" role="switch" aria-checked={repeat} aria-label="Repetir todo mês" onClick={() => setRepeat(!repeat)} />
        </div>
      )}

      {general && <p className="error" role="alert">{general}</p>}
      <div className="actions">
        <button className="btn full" disabled={busy}>{busy ? "Salvando…" : "Salvar"}</button>
        {!tx && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void save(true)}>Salvar e lançar outro</button>}
        {tx && (confirmDelete
          ? <button type="button" className="btn danger full" disabled={busy} onClick={() => void remove()}>Confirmar exclusão</button>
          : <button type="button" className="btn ghost full" onClick={() => setConfirmDelete(true)}>Excluir lançamento</button>)}
      </div>
    </form>
  );
}
