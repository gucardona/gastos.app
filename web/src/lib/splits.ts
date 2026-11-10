/** "33,33" → 3333 basis points; null if it is not a valid number. */
export const toBp = (s: string) => {
  const v = s.trim().replace(",", ".");
  if (v === "") return 0;
  if (!/^\d+(\.\d{1,2})?$/.test(v)) return null;
  return Math.round(Number(v) * 100);
};

/** Percentages typed per member → API shares. Null if anything is invalid or the total is not 100%. */
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

/** 3333 basis points → "33,33"; no trailing zeros. */
export const bpToPercent = (bp: number) => (bp / 100).toFixed(2).replace(/\.?0+$/, "").replace(".", ",");

/** Equal split as percentage text; the leftover hundredths go to the first member. */
export function equalPercents(userIds: number[]) {
  const base = Math.floor(10000 / userIds.length);
  const rest = 10000 - base * userIds.length;
  return Object.fromEntries(userIds.map((id, i) => [id, bpToPercent(base + (i === 0 ? rest : 0))]));
}

/**
 * Percentages for `ids`: the members' default shares when every one has one (normalized to 100% like the
 * server does: largest remainder, ties to the lowest userId), otherwise an equal split.
 */
export function defaultPercents(members: { userId: number; splitsByDefault: boolean; defaultShareBp: number }[], ids: number[]) {
  const sorted = [...ids].sort((a, b) => a - b);
  const shares = sorted.map((id) => members.find((m) => m.userId === id)?.defaultShareBp ?? 0);
  if (shares.some((s) => s <= 0)) return equalPercents(ids);
  const sum = shares.reduce((a, b) => a + b, 0);
  const bps = shares.map((s) => Math.floor((10000 * s) / sum));
  let left = 10000 - bps.reduce((a, b) => a + b, 0);
  shares
    .map((s, i) => ({ i, r: (10000 * s) % sum }))
    .sort((a, b) => b.r - a.r || a.i - b.i)
    .forEach(({ i }) => { if (left-- > 0) bps[i]++; });
  return Object.fromEntries(sorted.map((id, i) => [id, bpToPercent(bps[i])]));
}

/** Stored parts → percentage text per member, for the editor. */
export const percentsFromSplits = (splits: { userId: number; shareBp: number }[]) =>
  Object.fromEntries(splits.map((s) => [s.userId, bpToPercent(s.shareBp)]));
