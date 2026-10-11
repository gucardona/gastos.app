// Calendar dates ("YYYY-MM-DD") and months ("YYYY-MM") as strings, always in the local calendar.

const pad = (n: number) => String(n).padStart(2, "0");

export const toISODate = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
export const today = () => toISODate(new Date());
export const monthOf = (isoDate: string) => isoDate.slice(0, 7);

// noon keeps daylight-saving changes from shifting the day
const parse = (isoDate: string) => new Date(`${isoDate}T12:00:00`);

export function monthRange(month: string) {
  const [y, m] = month.split("-").map(Number);
  return { from: `${month}-01`, to: toISODate(new Date(y, m, 0, 12)) };
}

export function addDays(isoDate: string, delta: number) {
  const d = parse(isoDate);
  d.setDate(d.getDate() + delta);
  return toISODate(d);
}

export function addMonths(month: string, delta: number) {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(y, m - 1 + delta, 1, 12);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
}

export const monthLabel = (month: string) =>
  parse(`${month}-01`).toLocaleDateString("pt-BR", { month: "long", year: "numeric" });

/** "Hoje, 11 de outubro", "Ontem, 10 de outubro", "Sexta, 9 de outubro" (pt-BR copy shown in the UI). */
export function dayLabel(isoDate: string, todayISO = today()) {
  const d = parse(isoDate);
  const rest = d.toLocaleDateString("pt-BR", { day: "numeric", month: "long" });
  const diff = Math.round((parse(todayISO).getTime() - d.getTime()) / 86_400_000);
  if (diff === 0) return `Hoje, ${rest}`;
  if (diff === 1) return `Ontem, ${rest}`;
  const weekday = d.toLocaleDateString("pt-BR", { weekday: "long" });
  return `${weekday[0].toUpperCase()}${weekday.slice(1).replace("-feira", "")}, ${rest}`;
}
