const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

export const formatBRL = (cents: number) => brl.format(cents / 100);

/** Reais with thousands separators and cents split apart, for the highlighted balance. */
export function splitBRL(cents: number) {
  const sign = cents < 0 ? "-" : "";
  const abs = Math.abs(cents);
  const reais = Math.floor(abs / 100).toLocaleString("pt-BR");
  return { reais: sign + reais, cents: String(abs % 100).padStart(2, "0") };
}

/**
 * Parses what the person typed ("12,90", "1.234,56", "12.5") and returns whole cents.
 * Returns null for empty, zero, negative or more than 2 decimal places.
 */
export function parseCents(input: string): number | null {
  const s = input.replace(/R\$|\s/g, "");
  if (!/^\d[\d.,]*$/.test(s)) return null;

  let int: string;
  let frac = "";
  const comma = s.lastIndexOf(",");
  if (comma >= 0) {
    int = s.slice(0, comma).replace(/\./g, "");
    frac = s.slice(comma + 1);
    if (s.indexOf(",") !== comma) return null; // more than one comma
  } else {
    const dot = s.lastIndexOf(".");
    // a single dot followed by 1–2 digits is a decimal separator; anything else is a thousands separator
    if (dot >= 0 && s.indexOf(".") === dot && s.length - dot - 1 <= 2) {
      int = s.slice(0, dot);
      frac = s.slice(dot + 1);
    } else {
      int = s.replace(/\./g, "");
    }
  }
  if (!/^\d*$/.test(frac) || frac.length > 2) return null;
  const cents = Number(int || "0") * 100 + Number(frac.padEnd(2, "0") || "0");
  return cents > 0 ? cents : null;
}

/**
 * An amount field fills from the cents, like a card machine: the digits typed so far are the cents,
 * so "587" reads "5,87" and "58700" reads "587,00". Non-digits are ignored, at most 12 digits.
 */
export function typedAmount(raw: string): string {
  const digits = raw.replace(/\D/g, "").replace(/^0+/, "").slice(0, 12);
  if (!digits) return "";
  const padded = digits.padStart(3, "0");
  const reais = padded.slice(0, -2).replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${reais},${padded.slice(-2)}`;
}

/** Cents as an amount field shows them: 12990 becomes "129,90". */
export const centsToInput = (cents: number) => typedAmount(String(cents));

/** Why an amount cannot be saved (pt-BR copy shown in the UI), or "" when it can. */
export function amountProblem(input: string): string {
  if (parseCents(input)) return "";
  return /^\d[\d.]*,\d{3,}$/.test(input.replace(/R\$|\s/g, "")) ? "Use até 2 casas decimais." : "Informe um valor maior que zero.";
}
