import { useState } from "react";
import { Link } from "react-router";
import { useSummary, useTransactions } from "../api/queries";
import type { Transaction, Transfer, Upcoming } from "../api/types";
import { Flows } from "../components/Flows";
import { MonthSwitcher, PageBar } from "../components/PageBar";
import { SettlementSheet } from "../components/SettlementSheet";
import { TransactionList } from "../components/TransactionList";
import { UpcomingList } from "../components/UpcomingList";
import { firstName, useApp } from "../context";
import { monthOf, monthRange, today } from "../lib/dates";
import { formatBRL, splitBRL } from "../lib/money";
import { groupTop, pctLabel } from "../lib/share";
import { Chart } from "../components/Chart";

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
  const expenses = summary.byCategory.filter((c) => c.kind === "expense");
  const expenseTotal = expenses.reduce((s, c) => s + c.cents, 0);
  const pie = groupTop(expenses.map((c) => ({ label: categories.find((x) => x.id === c.categoryId)?.name ?? "?", value: c.cents })), 7, "Outros");
  const now = today();
  const realized = txs.filter((t) => t.date <= now); // a transaction dated after today is still to come: it shows in Próximos
  const empty = realized.length === 0 && summary.byCategory.length === 0 && summary.upcoming.length === 0;
  const forecast = summary.balanceCents + summary.upcomingIncomeCents - summary.upcomingExpenseCents;

  return (
    <>
      <section className="hero">
        <div className="hero-main">
          <div className="label">Saldo do mês</div>
          <p className={`balance num ${summary.balanceCents < 0 ? "negative" : ""}`}>
            <span className="cur">R$</span>{reais}<span className="cents">,{cents}</span>
          </p>
          {summary.upcoming.length > 0 && (
            <p className="forecast">Previsto para o fim do mês: <b className={`num ${forecast < 0 ? "negative" : ""}`}>{formatBRL(forecast)}</b></p>
          )}
        </div>
        <Flows income={summary.incomeCents} expense={summary.expenseCents} toReceive={summary.upcomingIncomeCents} toPay={summary.upcomingExpenseCents} />
      </section>

      {empty ? (
        <div className="empty">
          <p>Nada lançado em {wallet.name} neste mês. Use o botão + para registrar o primeiro gasto ou entrada.</p>
        </div>
      ) : (
        <>
          <Ruler month={month} txs={realized} summary={summary} />
          <div className="cols">
            {summary.byCategory.some((c) => c.kind === "expense") && (
            <section className="sec">
              <h2>Para onde foi</h2>
              <p className="sub">A linha vertical marca o orçamento do mês.</p>
              <div className="list-group">
              {summary.byCategory.filter((c) => c.kind === "expense").map((c) => {
                const cat = categories.find((x) => x.id === c.categoryId);
                const over = c.budgetCents !== null && c.cents > c.budgetCents;
                const scale = Math.max(c.cents, c.budgetCents ?? 0, 1);
                // with a budget: spent against the budget; without one: the share of the month's expenses, as the percentage says
                const fill = c.budgetCents !== null ? c.cents / scale : c.cents / Math.max(expenseTotal, 1);
                return (
                  <div className="rail" key={c.categoryId}>
                    <div className="rail-top">
                      <span className="name"><span aria-hidden>{cat?.icon}</span><span className="name-text">{cat?.name}</span></span>
                      <span className="amt num">{formatBRL(c.cents)} · {pctLabel(c.cents, expenseTotal)}{c.budgetCents !== null && <> <small>de {formatBRL(c.budgetCents)}</small></>}</span>
                    </div>
                    <div className={`track ${over ? "over" : ""}`}>
                      <div className="fill" style={{ width: `${fill * 100}%`, minWidth: c.budgetCents === null && c.cents > 0 ? 2 : undefined }} />
                      {c.budgetCents !== null && <div className="limit" style={{ left: `calc(${(c.budgetCents / scale) * 100}% - 1px)` }} />}
                    </div>
                    {over && <div className="warn">{formatBRL(c.cents - c.budgetCents!)} acima do orçamento</div>}
                  </div>
                );
              })}
              </div>
              {expenseTotal > 0 && <div className="list-group chart-card"><Chart chart={{ kind: "pie", title: "Gastos por categoria", points: pie }} /></div>}
            </section>
            )}
            <div>
              <Settle summary={summary} />
              {realized.length > 0 && (
              <section className="sec">
                <h2>Recentes</h2>
                <TransactionList items={realized.slice(0, 6)} headingLevel={3} />
                {realized.length > 6 && <p style={{ marginTop: 14 }}><Link className="link" to={`/lancamentos?m=${month}`}>Ver todos os lançamentos</Link></p>}
              </section>
              )}
              {summary.upcoming.length > 0 && (
              <section className="sec">
                <h2>Próximos</h2>
                <UpcomingList items={summary.upcoming} txs={txs} />
              </section>
              )}
            </div>
          </div>
        </>
      )}
    </>
  );
}

/** The month as a ruler: one tick per day, the future dotted. */
function Ruler({ month, txs, summary }: { month: string; txs: Transaction[]; summary: { dailyAverageCents: number; previous: { expenseCents: number }; upcoming: Upcoming[]; upcomingExpenseCents: number } }) {
  const days = Number(monthRange(month).to.slice(8));
  const now = today();
  const todayMonth = monthOf(now);
  const todayDay = month === todayMonth ? Number(now.slice(8)) : month < todayMonth ? days : 0;

  const spent = Array<number>(days).fill(0);
  for (const t of txs) if (t.kind === "expense") spent[Number(t.date.slice(8)) - 1] += t.amountCents;
  const projected = Array<number>(days).fill(0); // upcoming entries are always after today
  for (const u of summary.upcoming) if (u.kind === "expense") projected[Number(u.date.slice(8)) - 1] += u.amountCents;
  const max = Math.max(...spent, ...projected, 1);
  const height = (v: number) => Math.max(6, Math.round(Math.sqrt(v / max) * RULER_HEIGHT));

  return (
    <section className="strip" aria-label="Gastos por dia">
      <div className="ruler" style={{ gridTemplateColumns: `repeat(${days}, 1fr)` }}>
        {spent.map((v, i) => {
          const d = i + 1;
          if (d > todayDay) {
            const p = projected[i];
            return p > 0 ? <i key={d} className="future projected" style={{ height: height(p) }} title={`Dia ${d}: ${formatBRL(p)} previsto`} /> : <i key={d} className="future" />;
          }
          if (v === 0) return <i key={d} className="zero" title={`Dia ${d}: sem gastos`} />;
          return <i key={d} className={d === todayDay && month === todayMonth ? "today" : ""} style={{ height: height(v) }} title={`Dia ${d}: ${formatBRL(v)}`} />;
        })}
      </div>
      <div className="ruler-axis" style={{ gridTemplateColumns: `repeat(${days}, 1fr)` }} aria-hidden>
        {spent.map((_, i) => {
          const d = i + 1;
          const isToday = month === todayMonth && d === todayDay;
          return <span key={d} className={isToday ? "today" : ""}>{[1, 5, 10, 15, 20, 25, 30].includes(d) || isToday ? d : ""}</span>;
        })}
      </div>
      <p className="visually-hidden">
        {spent.map((v, i) => (v > 0 && i < todayDay ? `Dia ${i + 1}: ${formatBRL(v)}.` : "")).filter(Boolean).join(" ") || "Sem gastos até agora."}
        {summary.upcomingExpenseCents > 0 && ` Previsto: ${formatBRL(summary.upcomingExpenseCents)} a pagar (${projected.map((p, i) => (p > 0 ? `dia ${i + 1}: ${formatBRL(p)}` : "")).filter(Boolean).join("; ")}).`}
      </p>
      <p className="strip-note">
        {summary.dailyAverageCents > 0 && <>Média de <b className="num">{formatBRL(summary.dailyAverageCents)}</b> por dia. </>}
        {summary.previous.expenseCents > 0 && <>O mês anterior fechou em <span className="num">{formatBRL(summary.previous.expenseCents)}</span>.</>}
      </p>
    </section>
  );
}

function Settle({ summary }: { summary: { suggestedSettlements?: Transfer[] } }) {
  const { user, wallet, members, canEdit } = useApp();
  const [sheet, setSheet] = useState<Transfer | null>(null);
  const t = summary.suggestedSettlements?.[0];
  if (!wallet.splittingEnabled) return null;

  const who = (id: number) => (id === user.id ? "Você" : firstName(members.find((m) => m.userId === id)?.name ?? "Alguém"));

  return (
    <section className="sec" style={{ marginTop: 36 }}>
      <h2>Acerto entre vocês</h2>
      <p className="sub">Com base nos gastos divididos até hoje.</p>
      <div className="settle">
        {t ? (
          <>
            <p><span className="big num">{formatBRL(t.amountCents)}</span></p>
            <p>{who(t.fromUserId)} deve esse valor {t.toUserId === user.id ? "a você" : `para ${who(t.toUserId)}`}.</p>
            {canEdit && <button type="button" className="btn full" onClick={() => setSheet(t)}>Registrar acerto</button>}
          </>
        ) : (
          <p>Vocês estão quites.</p>
        )}
      </div>
      <SettlementSheet settlement={sheet} onClose={() => setSheet(null)} />
    </section>
  );
}
