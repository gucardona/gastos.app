import { useState } from "react";
import { api } from "../../api/client";
import { useBudgets } from "../../api/queries";
import type { Category } from "../../api/types";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";
import { parseCents } from "../../lib/money";

const centsToInput = (c: number) => (c / 100).toFixed(2).replace(".", ",");

export function BudgetsSection() {
  const { wallet, categories } = useApp();
  const budgets = useBudgets(wallet.id);
  const expense = categories.filter((c) => c.kind === "expense" && !c.archived);

  if (budgets.error) return <div className="empty"><p>Não foi possível carregar os orçamentos.</p><button className="btn" onClick={() => void budgets.refetch()}>Tentar de novo</button></div>;
  if (!budgets.data) return <div className="skeleton" />;
  return (
    <div className="settings">
      <p className="muted" style={{ margin: "0 0 8px" }}>O limite mensal de cada categoria aparece como uma linha na Visão geral.</p>
      {expense.map((c) => <Row key={c.id} c={c} current={budgets.data.find((b) => b.categoryId === c.id)?.limitCents} />)}
    </div>
  );
}

function Row({ c, current }: { c: Category; current?: number }) {
  const { wallet, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const [value, setValue] = useState(current ? centsToInput(current) : "");
  const [problem, setProblem] = useState("");
  const path = `/wallets/${wallet.id}/budgets/${c.id}`;

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(value);
    if (!cents) return setProblem("Informe um valor maior que zero.");
    setProblem("");
    await run(() => api.put(path, { limitCents: cents }));
  }

  return (
    <form className="line" onSubmit={save}>
      <div><span aria-hidden>{c.icon}</span> {c.name}{(problem || error) && <small className="error" role="alert">{problem || error}</small>}</div>
      <div className="item-actions" style={{ margin: 0 }}>
        <span className="muted">R$</span>
        <input className="text" style={{ width: 96, textAlign: "right" }} inputMode="decimal" placeholder="sem limite" aria-label={`Limite mensal de ${c.name}`} value={value} disabled={!canEdit} onChange={(e) => setValue(e.target.value)} />
        {canEdit && <button className="chip" disabled={busy || !value.trim()}>Definir</button>}
        {canEdit && current !== undefined && (
          <button type="button" className="chip" disabled={busy} onClick={async () => { if (await run(() => api.del(path))) setValue(""); }}>Remover</button>
        )}
      </div>
    </form>
  );
}
