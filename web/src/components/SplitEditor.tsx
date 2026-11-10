import { useEffect } from "react";
import { useApp } from "../context";
import { formatBRL } from "../lib/money";
import { defaultPercents, sharesFromPercents } from "../lib/splits";
import { ListGroup, ListRow } from "./ListGroup";

export type Percents = Record<number, string>;

/** The API shares for the typed percentages (undefined = the default split), or the message when they do not add up. */
export function resolveSplits(percents: Percents | null) {
  if (!percents) return {};
  const splits = sharesFromPercents(percents);
  return splits ? { splits } : { error: "As partes precisam somar 100%." };
}

/** "Dividir com ..." row with its Ajustar/Igualar chip and the percent inputs; `value` null = the wallet's default split. */
export function SplitEditor({ value, onChange, paidBy, cents, error }: {
  value: Percents | null;
  onChange: (v: Percents | null) => void;
  paidBy: number;
  cents: number | null;
  error?: string;
}) {
  const { members } = useApp();
  const defaultIds = members.filter((m) => m.splitsByDefault).map((m) => m.userId);
  const nameOf = (id: number) => members.find((m) => m.userId === id)?.name.split(" ")[0] ?? "";
  const byShare = defaultIds.length > 0 && defaultIds.every((id) => (members.find((m) => m.userId === id)?.defaultShareBp ?? 0) > 0);
  // a member removed meanwhile (live updates) must not leave a stale percentage behind
  useEffect(() => {
    if (value && Object.keys(value).some((id) => !members.some((m) => m.userId === Number(id)))) {
      onChange(Object.fromEntries(Object.entries(value).filter(([id]) => members.some((m) => m.userId === Number(id)))));
    }
  }, [members, value, onChange]);
  const summary = byShare
    ? Object.entries(defaultPercents(members, defaultIds)).map(([id, p]) => `${nameOf(Number(id))} ${p}%`).join(", ")
    : cents ? `${formatBRL(Math.floor(cents / Math.max(defaultIds.length, 1)))} para cada` : undefined;

  return (
    <>
      <ListGroup className="split-group">
        <ListRow
          label={value ? "Divisão personalizada" : `Dividir com ${defaultIds.filter((id) => id !== paidBy).map(nameOf).join(", ") || "ninguém"}`}
          value={value ? undefined : summary}
          trailing={
            <button type="button" className="chip" onClick={() => onChange(value ? null : defaultPercents(members, defaultIds.length ? defaultIds : members.map((m) => m.userId)))}>
              {value ? (byShare ? "Padrão" : "Igualar") : "Ajustar"}
            </button>
          }
        />
        {value && (
          <div className="split-edit">
            {members.map((m) => (
              <label key={m.userId}>
                {nameOf(m.userId)}
                <span><input inputMode="decimal" aria-label={`Porcentagem de ${m.name}`} value={value[m.userId] ?? ""} onChange={(e) => onChange({ ...value, [m.userId]: e.target.value })} /> %</span>
              </label>
            ))}
          </div>
        )}
      </ListGroup>
      {error && <p className="error" role="alert">{error}</p>}
    </>
  );
}
