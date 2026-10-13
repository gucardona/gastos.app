import { useState } from "react";
import { Link } from "react-router";
import { ApiError, api } from "../api/client";
import { useRefresh, useSummary, useTransactions } from "../api/queries";
import type { Transaction } from "../api/types";
import { MonthSwitcher, PageBar } from "../components/PageBar";
import { TransactionList } from "../components/TransactionList";
import { firstName, useApp } from "../context";
import { monthOf, monthRange, today } from "../lib/dates";
import { formatBRL, splitBRL } from "../lib/money";

const RULER_HEIGHT = 72;

export function Overview() {
  const { wallet, month } = useApp();
  const summary = useSummary(wallet.id, month);
  const txs = useTransactions(wallet.id, month);

  return (
    <>
      <PageBar />
      <MonthSwitcher />
      {summary.error || txs.error ? (
        <div className="empty">
          <p>Não foi possível carregar este mês.</p>
          <button type="button" className="btn" onClick={() => { void summary.refetch(); void txs.refetch(); }}>Tentar de novo</button>
        </div>
      ) : !summary.data || !txs.data ? (
        <><div className="skeleton" style={{ height: 56, width: "55%" }} /><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></>
      ) : (
        <Content month={month} summary={summary.data} txs={txs.data} />
      )}
    </>
  );
}

function Content({ month, summary, txs }: { month: string; summary: NonNullable<ReturnType<typeof useSummary>["data"]>; txs: Transaction[] }) {
  const { wallet, categories } = useApp();
  const { reais, cents } = splitBRL(summary.balanceCents);
  const empty = txs.length === 0 && summary.byCategory.length === 0;

  return (
    <>
      <section className="hero">
        <div className="hero-main">
          <div className="label">Saldo do mês</div>
          <p className={`balance num ${summary.balanceCents < 0 ? "negative" : ""}`}>
            <span className="cur">R$</span>{reais}<span className="cents">,{cents}</span>
          </p>
        </div>
        <div className="flows">
          <span className="in">Entrou <b className="num">{formatBRL(summary.incomeCents)}</b></span>
          <span>Saiu <b className="num">{formatBRL(summary.expenseCents)}</b></span>
        </div>
      </section>

      {empty ? (
        <div className="empty">
          <p>Nada lançado em {wallet.name} neste mês. Use o botão + para registrar o primeiro gasto ou entrada.</p>
        </div>
      ) : (
        <>
          <Ruler month={month} txs={txs} summary={summary} />
          <div className="cols">
            <section className="sec">
              <h2>Para onde foi</h2>
              <p className="sub">A linha vertical marca o orçamento do mês.</p>
              <div className="list-group">
              {summary.byCategory.filter((c) => c.kind === "expense").map((c) => {
                const cat = categories.find((x) => x.id === c.categoryId);
                const over = c.budgetCents !== null && c.cents > c.budgetCents;
                const scale = Math.max(c.cents, c.budgetCents ?? 0, 1);
                const topExpense = Math.max(...summary.byCategory.filter((x) => x.kind === "expense").map((x) => x.cents), 1);
                return (
                  <div className="rail" key={c.categoryId}>
                    <div className="rail-top">
                      <span className="name"><span aria-hidden>{cat?.icon}</span>{cat?.name}</span>
                      <span className="amt num">{formatBRL(c.cents)}{c.budgetCents !== null && <> <small>de {formatBRL(c.budgetCents)}</small></>}</span>
                    </div>
                    <div className={`track ${over ? "over" : ""}`}>
                      <div className="fill" style={{ width: `${(c.cents / (c.budgetCents !== null ? scale : topExpense)) * 100}%` }} />
                      {c.budgetCents !== null && <div className="limit" style={{ left: `calc(${(c.budgetCents / scale) * 100}% - 1px)` }} />}
                    </div>
                    {over && <div className="warn">{formatBRL(c.cents - c.budgetCents!)} acima do orçamento</div>}
                  </div>
                );
              })}
              </div>
            </section>
            <div>
              <Settle summary={summary} />
              <section className="sec">
                <h2>Recentes</h2>
                <TransactionList items={txs.slice(0, 6)} />
                {txs.length > 6 && <p style={{ marginTop: 14 }}><Link className="link" to={`/lancamentos?m=${month}`}>Ver todos os lançamentos</Link></p>}
              </section>
            </div>
          </div>
        </>
      )}
    </>
  );
}

/** The month as a ruler: one tick per day, the future dotted. */
function Ruler({ month, txs, summary }: { month: string; txs: Transaction[]; summary: { dailyAverageCents: number; previous: { expenseCents: number } } }) {
  const days = Number(monthRange(month).to.slice(8));
  const now = today();
  const todayMonth = monthOf(now);
  const todayDay = month === todayMonth ? Number(now.slice(8)) : month < todayMonth ? days : 0;

  const spent = Array<number>(days).fill(0);
  for (const t of txs) if (t.kind === "expense") spent[Number(t.date.slice(8)) - 1] += t.amountCents;
  const max = Math.max(...spent, 1);

  return (
    <section className="strip" aria-label="Gastos por dia">
      <div className="ruler" style={{ gridTemplateColumns: `repeat(${days}, 1fr)` }}>
        {spent.map((v, i) => {
          const d = i + 1;
          if (d > todayDay) return <i key={d} className="future" />;
          if (v === 0) return <i key={d} className="zero" title={`Dia ${d}: sem gastos`} />;
          return <i key={d} className={d === todayDay && month === todayMonth ? "today" : ""} style={{ height: Math.max(6, Math.round(Math.sqrt(v / max) * RULER_HEIGHT)) }} title={`Dia ${d}: ${formatBRL(v)}`} />;
        })}
      </div>
      <div className="ruler-axis" style={{ gridTemplateColumns: `repeat(${days}, 1fr)` }} aria-hidden>
        {spent.map((_, i) => {
          const d = i + 1;
          const isToday = month === todayMonth && d === todayDay;
          return <span key={d} className={isToday ? "today" : ""}>{[1, 5, 10, 15, 20, 25, 30].includes(d) || isToday ? d : ""}</span>;
        })}
      </div>
      <p className="strip-note">
        {summary.dailyAverageCents > 0 && <>Média de <b className="num">{formatBRL(summary.dailyAverageCents)}</b> por dia. </>}
        {summary.previous.expenseCents > 0 && <>O mês anterior fechou em <span className="num">{formatBRL(summary.previous.expenseCents)}</span>.</>}
      </p>
    </section>
  );
}

function Settle({ summary }: { summary: { suggestedSettlements?: { fromUserId: number; toUserId: number; amountCents: number }[] } }) {
  const { user, wallet, members, canEdit } = useApp();
  const refresh = useRefresh();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const t = summary.suggestedSettlements?.[0];
  if (!wallet.splittingEnabled) return null;

  const who = (id: number) => (id === user.id ? "Você" : firstName(members.find((m) => m.userId === id)?.name ?? "Alguém"));

  async function register() {
    if (!t) return;
    setBusy(true);
    setError("");
    try {
      await api.post(`/wallets/${wallet.id}/settlements`, { fromUserId: t.fromUserId, toUserId: t.toUserId, amountCents: t.amountCents, date: today() });
      refresh(wallet.id);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Sem conexão. Tente de novo.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="sec" style={{ marginTop: 36 }}>
      <h2>Acerto entre vocês</h2>
      <p className="sub">Com base nos gastos divididos até hoje.</p>
      <div className="settle">
        {t ? (
          <>
            <p><span className="big num">{formatBRL(t.amountCents)}</span></p>
            <p>{who(t.fromUserId)} deve esse valor {t.toUserId === user.id ? "a você" : `para ${who(t.toUserId)}`}.</p>
            {canEdit && <button type="button" className="btn full" disabled={busy} onClick={() => void register()}>Registrar acerto</button>}
            {error && <p className="error" role="alert">{error}</p>}
          </>
        ) : (
          <p>Vocês estão quites.</p>
        )}
      </div>
    </section>
  );
}
