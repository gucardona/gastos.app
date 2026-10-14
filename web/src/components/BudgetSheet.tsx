import { useState } from "react";
import { api } from "../api/client";
import type { Category } from "../api/types";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { centsToInput, parseCents } from "../lib/money";
import { Sheet } from "./Sheet";

/** The monthly limit of one expense category. `current` is the limit in cents, when there is one. Closed while `category` is null. */
export function BudgetSheet({ category, current, onClose }: { category: Category | null; current?: number; onClose: () => void }) {
  return (
    <Sheet open={category !== null} onClose={onClose} label="Limite mensal">
      {category && <BudgetForm key={category.id} category={category} current={current} onClose={onClose} />}
    </Sheet>
  );
}

function BudgetForm({ category, current, onClose }: { category: Category; current?: number; onClose: () => void }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [value, setValue] = useState(current ? centsToInput(current) : "");
  const [problem, setProblem] = useState("");
  const path = `/wallets/${wallet.id}/budgets/${category.id}`;

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(value);
    if (!cents) return setProblem("Informe um valor maior que zero.");
    setProblem("");
    if (await run(() => api.put(path, { limitCents: cents }))) onClose();
  }

  return (
    <form onSubmit={save}>
      <div className="sheet-head">
        <h2 className="sheet-title"><span aria-hidden="true">{category.icon}</span> {category.name}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <div className="field-row">
        <label htmlFor="budget-limit" className="lr-label">Limite mensal, R$</label>
        <input id="budget-limit" className="desc-input" inputMode="decimal" placeholder="sem limite" aria-label={`Limite mensal de ${category.name}`} value={value} onChange={(e) => { setValue(e.target.value); setProblem(""); }} />
      </div>
      {(problem || error) && <p className="error" role="alert">{problem || error}</p>}
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>Definir</button>
        {current !== undefined && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void run(() => api.del(path)).then((ok) => ok && onClose())}>Remover limite</button>}
      </div>
    </form>
  );
}
