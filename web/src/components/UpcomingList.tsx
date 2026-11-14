import { useState } from "react";
import { useRecurrences } from "../api/queries";
import type { Recurrence, Upcoming } from "../api/types";
import { useApp } from "../context";
import { dayLabel } from "../lib/dates";
import { RecurrenceSheet } from "./RecurrenceSheet";
import { money } from "./TransactionList";

const signed = (u: Upcoming) => (u.kind === "income" ? u.amountCents : -u.amountCents);

/** The month's recurring entries still to come, by day (soonest first); tapping one edits its recurrence. */
export function UpcomingList({ items }: { items: Upcoming[] }) {
  const { wallet, categories, paymentMethods, canEdit } = useApp();
  const recs = useRecurrences(wallet.id);
  const [editing, setEditing] = useState<Recurrence | null>(null);

  const days: { date: string; items: Upcoming[] }[] = [];
  for (const u of items) {
    const last = days[days.length - 1];
    if (last?.date === u.date) last.items.push(u);
    else days.push({ date: u.date, items: [u] });
  }

  return (
    <>
      {days.map((d) => (
        <section className="day" key={d.date}>
          <div className="day-head">
            <h3 className="day-title">{dayLabel(d.date)}</h3>
            <span className="num">{money(d.items.reduce((s, u) => s + signed(u), 0))}</span>
          </div>
          <div className="list-group">
          {d.items.map((u) => {
            const c = categories.find((x) => x.id === u.categoryId);
            const method = paymentMethods.find((p) => p.id === u.paymentMethodId)?.name;
            const rec = recs.data?.find((r) => r.id === u.recurrenceId);
            return (
              <button type="button" className="row upcoming" key={u.recurrenceId} disabled={!canEdit || !rec} onClick={() => setEditing(rec ?? null)} style={{ cursor: canEdit && rec ? "pointer" : "default" }}>
                <span className="emoji" aria-hidden>{c?.icon}</span>
                <span>
                  <span className="desc">{u.description}</span>
                  <span className="meta" style={{ display: "block" }}>
                    {[u.description !== c?.name ? c?.name : undefined, method].filter(Boolean).join(", ")} <span className="tag">previsto</span>
                  </span>
                </span>
                <span className={`val num ${u.kind === "income" ? "in" : ""}`}>{money(signed(u))}</span>
              </button>
            );
          })}
          </div>
        </section>
      ))}
      <RecurrenceSheet recurrence={editing} onClose={() => setEditing(null)} />
    </>
  );
}
