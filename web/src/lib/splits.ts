/** "33,33" → 3333 basis points; null se não for um número válido. */
const toBp = (s: string) => {
  const v = s.trim().replace(",", ".");
  if (v === "") return 0;
  if (!/^\d+(\.\d{1,2})?$/.test(v)) return null;
  return Math.round(Number(v) * 100);
};

/** Porcentagens digitadas por membro → partes da API. Null se algo é inválido ou a soma não fecha 100%. */
export function sharesFromPercents(percents: Record<number, string>) {
  const out: { userId: number; shareBp: number }[] = [];
  let total = 0;
  for (const [id, text] of Object.entries(percents)) {
    const bp = toBp(text);
    if (bp === null) return null;
    if (bp > 0) out.push({ userId: Number(id), shareBp: bp });
    total += bp;
  }
  return total === 10000 ? out : null;
}

/** Divisão igual em texto de porcentagem; o resto de centésimos vai para o primeiro. */
export function equalPercents(userIds: number[]) {
  const base = Math.floor(10000 / userIds.length);
  const rest = 10000 - base * userIds.length;
  return Object.fromEntries(
    userIds.map((id, i) => {
      const bp = base + (i === 0 ? rest : 0);
      const text = (bp / 100).toFixed(2).replace(/\.?0+$/, "").replace(".", ",");
      return [id, text];
    }),
  );
}
