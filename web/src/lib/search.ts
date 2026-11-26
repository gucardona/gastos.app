import type { Category, Member, PaymentMethod } from "../api/types";
import { formatBRL } from "./money";

/** Lowercase without accents, so "credito" finds "Crédito". */
export const fold = (s: string) => s.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();

export interface Searchable {
  amountCents: number;
  description: string;
  categoryId: number | null;
  paymentMethodId: number | null;
  date?: string | null;
  paidByUserId?: number;
}

interface Lookups {
  categories: Category[];
  paymentMethods: PaymentMethod[];
  members: Member[];
}

/** Everything a row shows or carries: names, the amount as people type it, the date as dd/mm and d/m, and who paid. */
function haystack(t: Searchable, { categories, paymentMethods, members }: Lookups) {
  const pm = paymentMethods.find((p) => p.id === t.paymentMethodId);
  const reais = (t.amountCents / 100).toFixed(2);
  const parts = [t.description, categories.find((c) => c.id === t.categoryId)?.name, pm?.name, pm?.card?.issuer, pm?.card?.last4,
    members.find((m) => m.userId === t.paidByUserId)?.name, formatBRL(t.amountCents), reais, reais.replace(".", ",")];
  if (t.date) {
    const [y, m, d] = t.date.split("-");
    parts.push(`${d}/${m}/${y}`, `${Number(d)}/${Number(m)}`);
  }
  return fold(parts.join(" ")).replace(/\s+/g, " ");
}

/** A predicate that keeps the items where every word of q appears somewhere in what the row shows. */
export function matcher(q: string, lookups: Lookups) {
  const words = fold(q).split(/\s+/).filter(Boolean);
  return (t: Searchable) => {
    if (words.length === 0) return true;
    const text = haystack(t, lookups);
    return words.every((w) => text.includes(w));
  };
}
