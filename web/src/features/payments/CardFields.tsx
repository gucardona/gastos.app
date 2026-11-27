import type { Card } from "../../api/types";
import { Segmented } from "../../ui/Segmented";

/** The card part of a payment method's form; days are kept as typed text until they are sent. */
export interface CardDraft {
  closingDay: string;
  dueDay: string;
  issuer: string;
  cardForm: Card["cardForm"];
  last4: string;
}

export const cardDraft = (c: Card | null): CardDraft => ({
  closingDay: c ? String(c.closingDay) : "", dueDay: c ? String(c.dueDay) : "",
  issuer: c?.issuer ?? "", cardForm: c?.cardForm ?? "", last4: c?.last4 ?? "",
});

/** What the API takes as `card`. */
export function cardBody(d: CardDraft) {
  const day = (s: string) => (s.trim() === "" ? null : Number(s));
  return { closingDay: day(d.closingDay), dueDay: day(d.dueDay), issuer: d.issuer, cardForm: d.cardForm, last4: d.last4 };
}

/** The first day of a new invoice: the day after the closing (31 wraps to 1), as the API computes it. */
export const bestPurchaseDay = (closingDay: number) => (closingDay % 31) + 1;

const digits = (s: string, max: number) => s.replace(/\D/g, "").slice(0, max);

export function CardFields({ value, onChange }: { value: CardDraft; onChange: (d: CardDraft) => void }) {
  const set = (patch: Partial<CardDraft>) => onChange({ ...value, ...patch });
  const closing = Number(value.closingDay);
  return (
    <>
      <div className="field-row">
        <label htmlFor="card-issuer" className="lr-label">Banco ou bandeira</label>
        <input id="card-issuer" className="desc-input" maxLength={60} placeholder="Opcional" value={value.issuer} onChange={(e) => set({ issuer: e.target.value })} />
      </div>
      <Segmented<Card["cardForm"]> label="Tipo do cartão" value={value.cardForm} onChange={(cardForm) => set({ cardForm: cardForm === value.cardForm ? "" : cardForm })}
        options={[{ value: "physical", label: "Físico" }, { value: "virtual", label: "Virtual" }]} />
      <div className="field-row">
        <label htmlFor="card-last4" className="lr-label">Final do cartão</label>
        <input id="card-last4" className="desc-input" inputMode="numeric" placeholder="4 últimos dígitos" value={value.last4} onChange={(e) => set({ last4: digits(e.target.value, 4) })} />
      </div>
      <div className="field-row">
        <label htmlFor="card-closing" className="lr-label">Compras até o dia</label>
        <input id="card-closing" className="desc-input" inputMode="numeric" value={value.closingDay} onChange={(e) => set({ closingDay: digits(e.target.value, 2) })} />
      </div>
      <div className="field-row">
        <label htmlFor="card-due" className="lr-label">Vence no dia</label>
        <input id="card-due" className="desc-input" inputMode="numeric" value={value.dueDay} onChange={(e) => set({ dueDay: digits(e.target.value, 2) })} />
      </div>
      {closing >= 1 && closing <= 31 && <p className="muted lead">Melhor dia de compra: {bestPurchaseDay(closing)}</p>}
    </>
  );
}
