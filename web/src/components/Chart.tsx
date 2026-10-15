import type { ReactNode } from "react";
import type { ChartData } from "../api/types";
import { formatBRL } from "../lib/money";

const W = 320;
const MAX_LABEL = 28;
// only theme tokens, so the slices follow light/dark
const PIE_COLORS = [
  "var(--accent)",
  "var(--income)",
  "var(--over)",
  "var(--muted)",
  "color-mix(in srgb, var(--accent) 55%, var(--ink))",
  "color-mix(in srgb, var(--income) 55%, var(--ink))",
  "color-mix(in srgb, var(--over) 55%, var(--ink))",
  "color-mix(in srgb, var(--accent) 45%, var(--muted))",
];

const clip = (s: string) => (s.length > MAX_LABEL ? `${s.slice(0, MAX_LABEL - 1)}…` : s);
const f1 = (n: number) => n.toFixed(1);

export function Chart({ chart }: { chart: ChartData }) {
  const { kind, title, points } = chart;
  const total = points.reduce((s, p) => s + p.value, 0);
  const pct = (v: number) => (total > 0 ? Math.round((v / total) * 100) : 0);
  const aria =
    `${title}. ` +
    points.map((p) => `${p.label}: ${formatBRL(p.value)}${kind === "pie" ? `, ${pct(p.value)}%` : ""}`).join("; ");
  const max = Math.max(1, ...points.map((p) => p.value));

  let body: ReactNode;
  if (points.length === 0) {
    return (
      <figure className="chart">
        <figcaption className="chart-title">{title}</figcaption>
        <p className="chart-empty">Sem dados</p>
      </figure>
    );
  } else if (kind === "bars") {
    const row = 36;
    body = (
      <svg viewBox={`0 0 ${W} ${points.length * row}`} role="img" aria-label={aria} className="chart-svg">
        {points.map((p, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static list, points have no id and labels may repeat
          <g key={i} transform={`translate(0 ${i * row})`}>
            <text x="0" y="12" className="chart-label">{clip(p.label)}</text>
            <text x={W} y="12" textAnchor="end" className="chart-value">{formatBRL(p.value)}</text>
            <rect className="chart-bar" x="0" y="18" height="12" rx="3" width={Math.max(2, (p.value / max) * W)} />
          </g>
        ))}
      </svg>
    );
  } else if (kind === "line") {
    const H = 140, pad = 10, base = H - 22;
    const x = (i: number) => (points.length === 1 ? W / 2 : pad + (i * (W - 2 * pad)) / (points.length - 1));
    const y = (v: number) => base - (v / max) * (base - 14);
    const first = points[0], last = points[points.length - 1];
    body = (
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={aria} className="chart-svg">
        <line className="chart-axis" x1="0" x2={W} y1={base} y2={base} />
        <polyline className="chart-line" fill="none" points={points.map((p, i) => `${f1(x(i))},${f1(y(p.value))}`).join(" ")} />
        {points.map((p, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static list, points have no id and labels may repeat
          <circle key={i} className="chart-dot" cx={f1(x(i))} cy={f1(y(p.value))} r="3" />
        ))}
        <text x="0" y={H - 6} className="chart-label">{clip(first.label)}</text>
        {points.length > 1 && <text x={W} y={H - 6} textAnchor="end" className="chart-label">{clip(last.label)}</text>}
        <text x="0" y="10" className="chart-value">máx. {formatBRL(max)}</text>
      </svg>
    );
  } else {
    const R = 90, C = 100;
    let angle = -Math.PI / 2;
    const shown = points.map((p, i) => ({ p, i })).filter(({ p }) => p.value > 0);
    const slices = shown.map(({ p, i }) => {
      const color = PIE_COLORS[i % PIE_COLORS.length];
      if (shown.length === 1) return <circle key={i} className="chart-slice" cx={C} cy={C} r={R} fill={color} />;
      const a0 = angle;
      angle += (p.value / total) * 2 * Math.PI;
      const pt = (a: number) => `${f1(C + R * Math.cos(a))} ${f1(C + R * Math.sin(a))}`;
      const large = angle - a0 > Math.PI ? 1 : 0;
      return <path key={i} className="chart-slice" fill={color} d={`M${C} ${C} L${pt(a0)} A${R} ${R} 0 ${large} 1 ${pt(angle)} Z`} />;
    });
    body = (
      <>
        <svg viewBox="0 0 200 200" role="img" aria-label={aria} className="chart-svg chart-pie">{slices}</svg>
        <ul className="chart-legend" aria-hidden="true">
          {points.map((p, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static list, points have no id and labels may repeat
            <li key={i}>
              <span className="chart-swatch" style={{ background: PIE_COLORS[i % PIE_COLORS.length] }} />
              <span className="chart-label">{p.label}</span>
              <span className="chart-value">{pct(p.value)}%</span>
            </li>
          ))}
        </ul>
      </>
    );
  }

  return (
    <figure className="chart">
      <figcaption className="chart-title">{title}</figcaption>
      {body}
    </figure>
  );
}
