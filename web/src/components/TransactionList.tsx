import type { Transaction } from "../api/types";
import { firstName, useApp } from "../context";
import { dayLabel } from "../lib/dates";
import { formatBRL } from "../lib/money";

const signed = (t: Transaction) => (t.kind === "income" ? t.amountCents : -t.amountCents);
const money = (cents: number) => `${cents < 0 ? "−" : "+"}${formatBRL(Math.abs(cents))}`;

/** Lançamentos agrupados por dia, na ordem recebida (a API já entrega do mais novo ao mais antigo). */
export function TransactionList({ items }: { items: Transaction[] }) {
  const { categories, paymentMethods, members, canEdit, openAdd } = useApp();
  const cat = (id: number) => categories.find((c) => c.id === id);
  const pm = (id: number | null) => paymentMethods.find((p) => p.id === id);
  const name = (id: number) => firstName(members.find((m) => m.userId === id)?.name ?? "Alguém");

  const days: { date: string; items: Transaction[] }[] = [];
  for (const t of items) {
    const last = days[days.length - 1];
    if (last?.date === t.date) last.items.push(t);
    else days.push({ date: t.date, items: [t] });
  }

  return (
    <>
      {days.map((d) => (
        <section className="day" key={d.date}>
          <div className="day-head">
            <strong>{dayLabel(d.date)}</strong>
            <span className="num">{money(d.items.reduce((s, t) => s + signed(t), 0))}</span>
          </div>
          {d.items.map((t) => {
            const c = cat(t.categoryId);
            const method = pm(t.paymentMethodId);
            const others = t.splits.filter((s) => s.userId !== t.paidByUserId).map((s) => name(s.userId));
            return (
              <button className="row" key={t.id} disabled={!canEdit} onClick={() => openAdd(t)} style={{ cursor: canEdit ? "pointer" : "default" }}>
                <span className="emoji" aria-hidden>{c?.icon}</span>
                <span>
                  <span className="desc">{t.description}</span>
                  <span className="meta" style={{ display: "block" }}>
                    {[c?.name, method?.name].filter(Boolean).join(", ")}
                    {t.recurrenceId && <> <span className="tag">recorrente</span></>}
                    {t.splits.length > 0 && <><br />{name(t.paidByUserId)} pagou{others.length > 0 && `, dividido com ${others.join(", ")}`}</>}
                  </span>
                </span>
                <span className={`val num ${t.kind === "income" ? "in" : ""}`}>{money(signed(t))}</span>
              </button>
            );
          })}
        </section>
      ))}
    </>
  );
}
