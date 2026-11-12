import { useEffect, useRef, useState } from "react";
import { ApiError, api, generalError } from "../api/client";
import { useRefresh } from "../api/queries";
import type { Kind, Transaction } from "../api/types";
import { useApp } from "../context";
import { useUndo } from "../hooks/useAction";
import { dayLabel, today, yesterday } from "../lib/dates";
import { amountProblem, centsToInput, parseCents, typedAmount } from "../lib/money";
import { percentsFromSplits } from "../lib/splits";
import { ListGroup, ListRow } from "./ListGroup";
import { CategoryGrid, DatePicker, OptionList, PickerView } from "./pickers";
import { Segmented } from "./Segmented";
import { Sheet } from "./Sheet";
import { resolveSplits, SplitEditor, type Percents } from "./SplitEditor";
import { Switch } from "./Switch";
import { useToast } from "./Toast";

/** Quick-add sheet: new transaction, or editing one when `state.tx` is set. */
export function AddSheet({ state, onClose }: { state: { tx?: Transaction } | null; onClose: () => void }) {
  return (
    <Sheet open={state !== null} onClose={onClose} label={state?.tx ? "Editar lançamento" : "Novo lançamento"}>
      <AddForm key={state?.tx?.id ?? "new"} tx={state?.tx} onClose={onClose} />
    </Sheet>
  );
}


type Picker = "category" | "date" | "method" | "paidBy";

function AddForm({ tx, onClose }: { tx?: Transaction; onClose: () => void }) {
  const { user, wallet, categories, paymentMethods, members } = useApp();
  const refresh = useRefresh();
  const toast = useToast();
  const undo = useUndo();

  const [kind, setKind] = useState<Kind>(tx?.kind ?? "expense");
  const [amount, setAmount] = useState(tx ? centsToInput(tx.amountCents) : "");
  const [description, setDescription] = useState(tx?.description ?? "");
  const [categoryId, setCategoryId] = useState<number | null>(tx?.categoryId ?? null);
  const [methodId, setMethodId] = useState<number | null>(tx?.paymentMethodId ?? null);
  const [date, setDate] = useState(tx?.date ?? today());
  const [paidBy, setPaidBy] = useState(tx?.paidByUserId ?? user.id);
  const [percents, setPercents] = useState<Percents | null>(tx?.splits.length ? percentsFromSplits(tx.splits) : null);
  const [repeat, setRepeat] = useState(false);
  const [picker, setPicker] = useState<Picker | null>(null);
  const openedFrom = useRef<Picker | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const amountRef = useRef<HTMLInputElement>(null);

  const cats = categories.filter((c) => c.kind === kind && (!c.archived || c.id === tx?.categoryId));
  const category = cats.find((c) => c.id === categoryId) ?? cats[0];
  const methods = paymentMethods.filter((p) => !p.archived || p.id === tx?.paymentMethodId);
  const method = methods.find((p) => p.id === methodId) ?? (tx ? undefined : methods[0]);

  const splitting = wallet.splittingEnabled && kind === "expense" && members.length > 1;
  const nameOf = (id: number) => members.find((m) => m.userId === id)?.name.split(" ")[0] ?? "";
  const cents = parseCents(amount);

  async function save(again: boolean) {
    const errs: Record<string, string> = {};
    if (!cents) errs.amountCents = amountProblem(amount);
    if (!category) errs.categoryId = "Escolha uma categoria.";
    const { splits, error: splitsError } = splitting ? resolveSplits(percents) : {};
    if (splitsError) errs.splits = splitsError;
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
      toast(!tx && repeat ? "Recorrência criada" : "Lançamento salvo");
      if (again) {
        setAmount("");
        setDescription("");
        setRepeat(false);
        amountRef.current?.focus();
      } else onClose();
    } catch (e) {
      if (e instanceof ApiError) {
        setErrors(e.fields);
        setGeneral(generalError(e, ["amountCents", "categoryId", "description", "date", ...(splitting ? ["splits"] : [])]));
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
      const { kind, amountCents, description, categoryId, paymentMethodId, date, paidByUserId } = tx;
      const again = { kind, amountCents, description, categoryId, paymentMethodId, date, paidByUserId, ...(tx.splits.length ? { splits: tx.splits.map((s) => ({ userId: s.userId, shareBp: s.shareBp })) } : {}) };
      // made by a recurrence: a copy would sit next to the occurrence the recurrence makes again, so no undo
      toast("Lançamento excluído", tx.recurrenceId === null ? { action: undo(() => api.post(`/wallets/${wallet.id}/transactions`, again)) } : undefined);
      onClose();
    } catch (e) {
      setGeneral(e instanceof ApiError ? e.message : "Sem conexão. Tente de novo.");
      setBusy(false);
    }
  }

  const err = (k: string) => errors[k] && <p id={`err-${k}`} className="error" role="alert">{errors[k]}</p>;
  /** Marks an input invalid and points it at its error, while it has one. */
  const invalid = (k: string) => (errors[k] ? { "aria-invalid": true, "aria-describedby": `err-${k}` } : {});

  // coming back from a picker, focus goes to the row that opened it
  useEffect(() => {
    if (picker === null && openedFrom.current) {
      document.getElementById(`row-${openedFrom.current}`)?.focus();
      openedFrom.current = null;
    }
  }, [picker]);
  const choose = (p: Picker) => { openedFrom.current = p; setPicker(p); };
  const back = () => setPicker(null);

  if (picker === "category") {
    return (
      <PickerView title="Categoria" onBack={back}>
        <CategoryGrid categories={cats} value={category?.id ?? null} onPick={(id) => { setCategoryId(id); back(); }} />
      </PickerView>
    );
  }
  if (picker === "date") {
    return <PickerView title="Quando" onBack={back}><DatePicker value={date} onPick={(iso) => { setDate(iso); if (iso === today() || iso === yesterday()) back(); }} /></PickerView>;
  }
  if (picker === "method") {
    return (
      <PickerView title="Pago com" onBack={back}>
        <OptionList options={methods.map((p) => ({ value: p.id, label: p.name }))} value={method?.id ?? null} onPick={(id) => { setMethodId(id); back(); }} />
      </PickerView>
    );
  }
  if (picker === "paidBy") {
    return (
      <PickerView title="Quem pagou" onBack={back}>
        <OptionList options={members.map((m) => ({ value: m.userId, label: m.name.split(" ")[0] }))} value={paidBy} onPick={(id) => { setPaidBy(id); back(); }} />
      </PickerView>
    );
  }

  const dateLabel = date === today() ? "Hoje" : date === yesterday() ? "Ontem" : dayLabel(date);

  return (
    <form onSubmit={(e) => { e.preventDefault(); void save(false); }}>
      <div className="sheet-head">
        <Segmented<Kind> label="Tipo" value={kind} onChange={setKind} options={[{ value: "expense", label: "Gasto" }, { value: "income", label: "Entrada" }]} />
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>

      <div className="amount">
        <span className="cur">R$</span>
        <input
          ref={amountRef} data-autofocus inputMode="numeric" enterKeyHint="done" placeholder="0,00" aria-label="Valor" value={amount} onChange={(e) => setAmount(typedAmount(e.target.value))} {...invalid("amountCents")}
          // grows with the amount; where CSS field-sizing is missing, a width in ch from the length of the text
          style={globalThis.CSS?.supports?.("field-sizing", "content") ? undefined : { width: `${Math.max(amount.length, 4) + 0.5}ch` }}
        />
      </div>
      {err("amountCents")}

      <ListGroup>
        <ListRow id="row-category" label="Categoria" value={category ? `${category.icon} ${category.name}` : "Escolha"} onClick={() => choose("category")} chevron />
        <ListRow id="row-date" label="Quando" value={dateLabel} onClick={() => choose("date")} chevron />
        {methods.length > 0 && <ListRow id="row-method" label="Pago com" value={method?.name ?? "Nenhuma"} onClick={() => choose("method")} chevron />}
        <div className="list-row">
          <label htmlFor="desc" className="lr-label">Descrição</label>
          <input id="desc" className="desc-input" placeholder="opcional" enterKeyHint="done" value={description} onChange={(e) => setDescription(e.target.value)} {...invalid("description")} />
        </div>
        {splitting && <ListRow id="row-paidBy" label="Quem pagou" value={nameOf(paidBy)} onClick={() => choose("paidBy")} chevron />}
        {!tx && <ListRow label="Repetir todo mês" trailing={<Switch checked={repeat} onChange={setRepeat} label="Repetir todo mês" />} />}
      </ListGroup>
      {err("categoryId")}
      {err("description")}
      {err("date")}

      {splitting && <SplitEditor value={percents} onChange={setPercents} paidBy={paidBy} cents={cents} error={errors.splits} />}

      {general && <p className="error" role="alert">{general}</p>}
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>{busy ? "Salvando…" : "Salvar"}</button>
        {!tx && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void save(true)}>Salvar e lançar outro</button>}
        {tx && (confirmDelete
          ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={(e) => e.detail < 2 && void remove()}>Confirmar exclusão</button>
          : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirmDelete(true)}>Excluir lançamento</button>)}
      </div>
    </form>
  );
}
