const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

export const formatBRL = (cents: number) => brl.format(cents / 100);

/** Reais com milhar e centavos separados, para o saldo em destaque. */
export function splitBRL(cents: number) {
  const sign = cents < 0 ? "-" : "";
  const abs = Math.abs(cents);
  const reais = Math.floor(abs / 100).toLocaleString("pt-BR");
  return { reais: sign + reais, cents: String(abs % 100).padStart(2, "0") };
}

/**
 * Lê o que a pessoa digitou ("12,90", "1.234,56", "12.5") e devolve centavos inteiros.
 * Devolve null para vazio, zero, negativo ou mais de 2 casas decimais.
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
    if (s.indexOf(",") !== comma) return null; // mais de uma vírgula
  } else {
    const dot = s.lastIndexOf(".");
    // um único ponto seguido de 1–2 dígitos é decimal; qualquer outro caso é separador de milhar
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
