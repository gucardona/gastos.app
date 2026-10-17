import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../../api/client";
import { useAISettings } from "../../api/queries";
import type { AISettings } from "../../api/types";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { Segmented } from "../../components/Segmented";
import { Switch } from "../../components/Switch";
import { useAction } from "../../hooks/useAction";
import { centsToInput, formatBRL, parseCents } from "../../lib/money";

const MODELS = [
  { value: "claude-haiku-5-5", label: "Haiku" },
  { value: "claude-sonnet-5-5", label: "Sonnet" },
  { value: "claude-opus-5-5", label: "Opus" },
];

const LIMIT_ERROR = "Use um valor de R$ 0,01 a R$ 100.000,00, ou deixe vazio para não ter limite.";
const usage = (when: string, used: number, limit: number) => `${when}: ${formatBRL(used)}${limit > 0 ? ` de ${formatBRL(limit)}` : ""}`;
const MAX_LIMIT_CENTS = 10_000_000;

/** Empty or zero means "no limit" (0); null when the text is not a valid amount. */
function parseLimit(text: string): number | null {
  const t = text.trim();
  if (t === "" || /^0+([.,]0*)?$/.test(t)) return 0;
  const c = parseCents(t);
  return c !== null && c <= MAX_LIMIT_CENTS ? c : null;
}

function LimitRow({ id, label, saveLabel, cents, busy, onInvalid, onSave }: {
  id: string; label: string; saveLabel: string; cents: number; busy: boolean;
  onInvalid: (message: string) => void; onSave: (cents: number) => Promise<boolean>;
}) {
  const [text, setText] = useState<string>();
  const current = cents > 0 ? centsToInput(cents) : "";
  const value = text ?? current;
  return (
    <form className="list-row" onSubmit={(e) => {
      e.preventDefault();
      const n = parseLimit(value);
      if (n === null) { onInvalid(LIMIT_ERROR); return; }
      onInvalid("");
      void onSave(n).then((ok) => ok && setText(undefined));
    }}>
      <label htmlFor={id} className="lr-label">{label} <span style={{ whiteSpace: "nowrap" }}>(R$) *</span></label>
      <input id={id} className="desc-input" type="text" inputMode="decimal" placeholder="sem limite" autoComplete="off" value={value} onChange={(e) => setText(e.target.value)} />
      {value !== current && <button type="submit" className="chip" disabled={busy}>{saveLabel}</button>}
    </form>
  );
}

export function AssistantSection() {
  const { data } = useAISettings();
  const qc = useQueryClient();
  const keyAct = useAction();
  const prefs = useAction();
  const [key, setKey] = useState("");
  const [replacing, setReplacing] = useState(false);
  const [days, setDays] = useState<string>();
  const [daysError, setDaysError] = useState("");
  const [warning, setWarning] = useState("");

  if (!data) return null;
  if (!data.available) return <p className="muted lead">O assistente não está disponível: o servidor não tem a chave mestra do assistente configurada.</p>;

  // useAction refreshes the wallet's data and ["me"]; the assistant settings and the chat switch live elsewhere
  const sync = () => Promise.all([qc.invalidateQueries({ queryKey: ["ai"] }), qc.invalidateQueries({ queryKey: ["me"] })]);
  const put = (act: typeof keyAct, body: object) => act.run(async () => {
    const res = await api.put<AISettings>("/me/ai", body);
    setWarning(res.warning ?? "");
    await sync();
  });

  const showKeyForm = !data.configured || replacing;
  const retention = days ?? String(data.retentionDays);

  return (
    <div className="screen">
      <ListGroup>
        {showKeyForm ? (
          <form className="list-row" onSubmit={async (e) => {
            e.preventDefault();
            if (await put(keyAct, { apiKey: key })) { setKey(""); setReplacing(false); }
          }}>
            <label htmlFor="ai-key" className="lr-label">Chave da API</label>
            <input id="ai-key" className="desc-input" type="password" autoComplete="off" value={key} onChange={(e) => setKey(e.target.value)} />
            <button type="submit" className="chip" disabled={keyAct.busy || !key.trim()}>Salvar chave</button>
          </form>
        ) : (
          <div className="list-row">
            <span className="lr-label">Chave da API</span> <span className="lr-value">•••• {data.keyHint}</span>
            <button type="button" className="chip" onClick={() => setReplacing(true)}>Trocar</button>
            <button type="button" className="chip" disabled={keyAct.busy} onClick={() => void keyAct.run(async () => { await api.del("/me/ai/key"); await sync(); })}>Remover</button>
          </div>
        )}
      </ListGroup>
      {keyAct.error && <p className="error" role="alert">{keyAct.error}</p>}
      {warning && <p className="muted lead" role="status">{warning}</p>}

      <Segmented label="Modelo" value={data.model} options={MODELS} onChange={(model) => void put(prefs, { model })} />
      <ListGroup>
        <ListRow label="Ativar chat" trailing={<Switch checked={data.enabled} disabled={!data.configured || prefs.busy} label="Ativar chat" onChange={(enabled) => void put(prefs, { enabled })} />} />
        <form className="list-row" onSubmit={(e) => {
          e.preventDefault();
          const n = Number(retention);
          if (!Number.isInteger(n) || n < 1 || n > 30) { setDaysError("Use de 1 a 30 dias."); return; }
          setDaysError("");
          void put(prefs, { retentionDays: n }).then((ok) => ok && setDays(undefined));
        }}>
          <label htmlFor="ai-days" className="lr-label">Guardar conversas por (dias)</label>
          <input id="ai-days" className="desc-input" type="number" inputMode="numeric" value={retention} onChange={(e) => setDays(e.target.value)} />
          {retention !== String(data.retentionDays) && <button type="submit" className="chip" disabled={prefs.busy}>Salvar prazo</button>}
        </form>
        <LimitRow id="ai-daily" label="Limite diário" saveLabel="Salvar limite diário" cents={data.dailyLimitCents} busy={prefs.busy}
          onInvalid={setDaysError} onSave={(dailyLimitCents) => put(prefs, { dailyLimitCents })} />
        <LimitRow id="ai-monthly" label="Limite mensal" saveLabel="Salvar limite mensal" cents={data.monthlyLimitCents} busy={prefs.busy}
          onInvalid={setDaysError} onSave={(monthlyLimitCents) => put(prefs, { monthlyLimitCents })} />
      </ListGroup>
      {(daysError || prefs.error) && <p className="error" role="alert">{daysError || prefs.error}</p>}
      <p className="muted lead">* Valor aproximado: o custo é estimado com a cotação do dólar e pode variar um pouco do que a Anthropic cobra. O limite real é o da sua conta no console da Anthropic.</p>
      <p className="muted lead">{usage("Hoje", data.usedTodayCents, data.dailyLimitCents)}</p>
      <p className="muted lead">{usage("Este mês", data.usedMonthCents, data.monthlyLimitCents)}</p>
      <p className="muted lead">
        {data.usdBrl !== null && data.usdBrlAt
          ? `Cotação usada: US$ 1 = ${formatBRL(Math.round(data.usdBrl * 100))} (atualizada em ${new Date(data.usdBrlAt).toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" })})`
          : "Cotação do dólar indisponível: os limites em reais só passam a valer quando ela for obtida."}
      </p>
      <p className="muted lead">Os dados dos lançamentos consultados são enviados à Anthropic com a sua chave.</p>
    </div>
  );
}
