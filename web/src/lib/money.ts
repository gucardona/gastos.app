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

/** Cents as the text a person types into an amount field: 12990 becomes "129,90". */
export const centsToInput = (cents: number) => (cents / 100).toFixed(2).replace(".", ",");
