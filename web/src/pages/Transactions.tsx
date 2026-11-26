import { useState } from "react";
import { useSummary, useTransactions } from "../api/queries";
import { Flows } from "../components/Flows";
import { MonthSwitcher, PageBar } from "../components/PageBar";
import { RecurringSection } from "../components/RecurringSection";
import { Segmented } from "../components/Segmented";
import { TransactionList } from "../components/TransactionList";
import { UpcomingList } from "../components/UpcomingList";
import { useApp } from "../context";
import { today } from "../lib/dates";
import { matcher, type Searchable } from "../lib/search";

const FILTERS = [
  { id: "all", label: "Tudo" },
  { id: "expense", label: "Gastos" },
  { id: "income", label: "Entradas" },
] as const;

type Filter = (typeof FILTERS)[number]["id"];

export function Transactions() {
  const { wallet, month, canEdit, openAdd, categories, paymentMethods, members } = useApp();
  const txs = useTransactions(wallet.id, month);
  const summary = useSummary(wallet.id, month);
  const [kind, setKind] = useState<Filter>("all");
  const [q, setQ] = useState("");

  const now = today();
  // a transaction dated after today is still to come: it shows in Próximos, not among the realized ones (as the summary counts)
  const all = (txs.data ?? []).filter((t) => t.date <= now);
  const found = matcher(q, { categories, paymentMethods, members });
  const matches = (t: { kind: string } & Searchable) => (kind === "all" || t.kind === kind) && found(t);
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
          <input className="search" type="search" placeholder="Buscar por descrição, valor, data, categoria, pagamento…" aria-label="Buscar" value={q} onChange={(e) => setQ(e.target.value)} />
          <Segmented<Filter> label="Filtros" value={kind} onChange={setKind} options={FILTERS.map((f) => ({ value: f.id, label: f.label }))} />
          {ahead.length > 0 && (
            <section className="sec">
              <h2>Próximos</h2>
              <UpcomingList items={ahead} txs={txs.data} />
            </section>
          )}
          {shown.length === 0 ? (
            <div className="empty">
              <p>{all.length === 0 ? "Nenhum lançamento neste mês." : "Nenhum lançamento corresponde ao filtro."}</p>
              {all.length === 0 && canEdit && <button type="button" className="btn" onClick={() => openAdd()}>Novo lançamento</button>}
            </div>
          ) : ahead.length > 0 ? (
            // after Próximos the realized list needs its own title, or it reads as more of what is to come
            <section className="sec">
              <h2>Lançados</h2>
              <TransactionList items={shown} headingLevel={3} />
            </section>
          ) : (
            <TransactionList items={shown} />
          )}
        </>
      )}
      <RecurringSection matches={q.trim() || kind !== "all" ? matches : undefined} />
    </>
  );
}
