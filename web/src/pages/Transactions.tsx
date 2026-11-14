import { useState } from "react";
import { useSummary, useTransactions } from "../api/queries";
import { Flows } from "../components/Flows";
import { MonthSwitcher, PageBar } from "../components/PageBar";
import { RecurringSection } from "../components/RecurringSection";
import { Segmented } from "../components/Segmented";
import { TransactionList } from "../components/TransactionList";
import { UpcomingList } from "../components/UpcomingList";
import { useApp } from "../context";

const FILTERS = [
  { id: "all", label: "Tudo" },
  { id: "expense", label: "Gastos" },
  { id: "income", label: "Entradas" },
] as const;

type Filter = (typeof FILTERS)[number]["id"];

/** Lowercase without accents, so "credito" finds "Crédito". */
const fold = (s: string) => s.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();

export function Transactions() {
  const { wallet, month, canEdit, openAdd, categories, paymentMethods } = useApp();
  const txs = useTransactions(wallet.id, month);
  const summary = useSummary(wallet.id, month);
  const [kind, setKind] = useState<Filter>("all");
  const [q, setQ] = useState("");

  const all = txs.data ?? [];
  const term = fold(q.trim());
  // a row with no description shows its category, so the category (and the payment method) can be searched too
  const text = (t: { description: string; categoryId: number | null; paymentMethodId: number | null }) => fold([t.description, categories.find((c) => c.id === t.categoryId)?.name, paymentMethods.find((p) => p.id === t.paymentMethodId)?.name].join(" "));
  const matches = (t: { kind: string } & Parameters<typeof text>[0]) => (kind === "all" || t.kind === kind) && (!term || text(t).includes(term));
  const shown = all.filter(matches);
  const ahead = (summary.data?.upcoming ?? []).filter(matches);
  const sum = (k: string) => all.filter((t) => t.kind === k).reduce((s, t) => s + t.amountCents, 0);

  return (
    <>
      <PageBar />
      <MonthSwitcher />
      {txs.error ? (
        <div className="empty">
          <p>Não foi possível carregar os lançamentos.</p>
          <button type="button" className="btn" onClick={() => void txs.refetch()}>Tentar de novo</button>
        </div>
      ) : !txs.data ? (
        <><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></>
      ) : (
        <>
          <Flows income={sum("income")} expense={sum("expense")} toReceive={summary.data?.upcomingIncomeCents ?? 0} toPay={summary.data?.upcomingExpenseCents ?? 0} />
          <input className="search" type="search" placeholder="Buscar por descrição ou categoria" aria-label="Buscar" value={q} onChange={(e) => setQ(e.target.value)} />
          <Segmented<Filter> label="Filtros" value={kind} onChange={setKind} options={FILTERS.map((f) => ({ value: f.id, label: f.label }))} />
          {ahead.length > 0 && (
            <section className="sec">
              <h2>Próximos</h2>
              <UpcomingList items={ahead} />
            </section>
          )}
          {shown.length === 0 ? (
            <div className="empty">
              <p>{all.length === 0 ? "Nenhum lançamento neste mês." : "Nenhum lançamento corresponde ao filtro."}</p>
              {all.length === 0 && canEdit && <button type="button" className="btn" onClick={() => openAdd()}>Novo lançamento</button>}
            </div>
          ) : (
            <TransactionList items={shown} />
          )}
        </>
      )}
      <RecurringSection />
    </>
  );
}
