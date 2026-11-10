/** A category's share of the total as text: integer percent, "<1%" when above zero but rounding to 0. */
export function pctLabel(value: number, total: number): string {
  if (total <= 0 || value <= 0) return "0%";
  const p = Math.round((value / total) * 100);
  return p === 0 ? "<1%" : `${p}%`;
}

type Point = { label: string; value: number };

/** Largest first; with more than top+1 points, the tail is summed into one `rest` point. */
export function groupTop(points: Point[], top: number, rest: string): Point[] {
  const sorted = [...points].sort((a, b) => b.value - a.value);
  if (sorted.length <= top + 1) return sorted;
  return [...sorted.slice(0, top), { label: rest, value: sorted.slice(top).reduce((s, p) => s + p.value, 0) }];
}
