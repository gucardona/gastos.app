import { useState } from "react";
import { useTransactions } from "../api/queries";
import { MonthSwitcher, PageBar } from "../components/PageBar";
import { TransactionList } from "../components/TransactionList";
import { useApp } from "../context";
import { formatBRL } from "../lib/money";

const FILTERS = [
  { id: "all", label: "Tudo" },
  { id: "expense", label: "Gastos" },
  { id: "income", label: "Entradas" },
] as const;

export function Transactions() {
  const { wallet, month, canEdit, openAdd } = useApp();
  const txs = useTransactions(wallet.id, month);
  const [kind, setKind] = useState<(typeof FILTERS)[number]["id"]>("all");
  const [q, setQ] = useState("");

  const all = txs.data ?? [];
  const term = q.trim().toLowerCase();
  const shown = all.filter((t) => (kind === "all" || t.kind === kind) && (!term || t.description.toLowerCase().includes(term)));
  const sum = (k: string) => all.filter((t) => t.kind === k).reduce((s, t) => s + t.amountCents, 0);

  return (
    <>
      <PageBar />
      <MonthSwitcher />
      {txs.error ? (
        <div className="empty">
          <p>Não foi possível carregar os lançamentos.</p>
          <button className="btn" onClick={() => void txs.refetch()}>Tentar de novo</button>
        </div>
      ) : !txs.data ? (
        <><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></>
      ) : (
        <>
          <div className="totals">
            <span>Entrou <b className="num">{formatBRL(sum("income"))}</b></span>
            <span>Saiu <b className="num">{formatBRL(sum("expense"))}</b></span>
          </div>
          <input className="search" type="search" placeholder="Buscar por descrição" aria-label="Buscar" value={q} onChange={(e) => setQ(e.target.value)} />
          <div className="filters" role="group" aria-label="Filtros">
            {FILTERS.map((f) => (
              <button key={f.id} className="chip" aria-pressed={kind === f.id} onClick={() => setKind(f.id)}>{f.label}</button>
            ))}
          </div>
          {shown.length === 0 ? (
            <div className="empty">
              <p>{all.length === 0 ? "Nenhum lançamento neste mês." : "Nenhum lançamento corresponde ao filtro."}</p>
              {all.length === 0 && canEdit && <button className="btn" onClick={() => openAdd()}>Novo lançamento</button>}
            </div>
          ) : (
            <TransactionList items={shown} />
          )}
        </>
      )}
    </>
  );
}
