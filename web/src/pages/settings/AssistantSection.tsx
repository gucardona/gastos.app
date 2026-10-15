import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../../api/client";
import { useAISettings } from "../../api/queries";
import type { AISettings } from "../../api/types";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { Segmented } from "../../components/Segmented";
import { Switch } from "../../components/Switch";
import { useAction } from "../../hooks/useAction";

const MODELS = [
  { value: "claude-haiku-5-5", label: "Haiku" },
  { value: "claude-sonnet-5-5", label: "Sonnet" },
  { value: "claude-opus-5-5", label: "Opus" },
];

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
      </ListGroup>
      {(daysError || prefs.error) && <p className="error" role="alert">{daysError || prefs.error}</p>}
      <p className="muted lead">Os dados dos lançamentos consultados são enviados à Anthropic com a sua chave.</p>
    </div>
  );
}
