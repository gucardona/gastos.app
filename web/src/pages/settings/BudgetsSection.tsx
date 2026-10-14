import { useState } from "react";
import { useBudgets } from "../../api/queries";
import type { Category } from "../../api/types";
import { BudgetSheet } from "../../components/BudgetSheet";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { useApp } from "../../context";
import { formatBRL } from "../../lib/money";

export function BudgetsSection() {
  const { wallet, categories, canEdit } = useApp();
  const budgets = useBudgets(wallet.id);
  const [open, setOpen] = useState<Category | null>(null);
  const expense = categories.filter((c) => c.kind === "expense" && !c.archived);

  if (budgets.error) return <div className="empty"><p>Não foi possível carregar os orçamentos.</p><button type="button" className="btn" onClick={() => void budgets.refetch()}>Tentar de novo</button></div>;
  if (!budgets.data) return <div className="skeleton" />;
  const limit = (id: number) => budgets.data.find((b) => b.categoryId === id)?.limitCents;

  return (
    <div className="screen">
      <p className="muted lead">O limite mensal de cada categoria aparece como uma linha na Visão geral.</p>
      <ListGroup>
        {expense.map((c) => (
          <ListRow key={c.id} icon={c.icon} label={c.name} value={limit(c.id) === undefined ? "sem limite" : formatBRL(limit(c.id) as number)} onClick={canEdit ? () => setOpen(c) : undefined} chevron={canEdit} />
        ))}
      </ListGroup>
      <BudgetSheet category={open} current={open ? limit(open.id) : undefined} onClose={() => setOpen(null)} />
    </div>
  );
}
