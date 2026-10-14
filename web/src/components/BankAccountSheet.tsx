import { useState } from "react";
import { api } from "../api/client";
import type { BankAccount } from "../api/types";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { ListGroup, ListRow } from "./ListGroup";
import { OptionList, PickerView } from "./pickers";
import { Segmented } from "./Segmented";
import { Sheet } from "./Sheet";
import { Switch } from "./Switch";

/** Create (`"new"`) or manage a bank account: payment method, on/off, importing a statement, deleting. Closed while `account` is null. */
export function BankAccountSheet({ account, onClose }: { account: BankAccount | "new" | null; onClose: () => void }) {
  return (
    <Sheet open={account !== null} onClose={onClose} label={account === "new" ? "Nova conta bancária" : "Conta bancária"}>
      {account === "new" && <NewAccount onClose={onClose} />}
      {account && account !== "new" && <Account key={account.id} account={account} onClose={onClose} />}
    </Sheet>
  );
}

function NewAccount({ onClose }: { onClose: () => void }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [name, setName] = useState("");
  return (
    <form onSubmit={async (e) => { e.preventDefault(); if (await run(() => api.post(`/wallets/${wallet.id}/bank-accounts`, { name }))) onClose(); }}>
      <div className="sheet-head">
        <h2 className="sheet-title">Nova conta bancária</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <div className="field-row">
        <label htmlFor="ba-name" className="lr-label">Nome</label>
        <input id="ba-name" className="desc-input" aria-label="Nome da conta bancária" value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions"><button type="submit" className="btn full" disabled={busy}>Criar</button></div>
    </form>
  );
}

function Account({ account, onClose }: { account: BankAccount; onClose: () => void }) {
  const { wallet, paymentMethods } = useApp();
  const { run, busy, error } = useAction();
  const [methodId, setMethodId] = useState(account.paymentMethodId);
  const [enabled, setEnabled] = useState(account.enabled);
  const [picking, setPicking] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const path = `/wallets/${wallet.id}/bank-accounts/${account.id}`;
  const save = (over: { paymentMethodId?: number | null; enabled?: boolean }) =>
    api.put(path, { name: account.name, paymentMethodId: methodId, enabled, ...over });
  const method = paymentMethods.find((p) => p.id === methodId);

  if (picking) {
    const options = [{ value: 0, label: "Nenhuma" }, ...paymentMethods.filter((p) => !p.archived || p.id === methodId).map((p) => ({ value: p.id, label: p.name }))];
    return (
      <PickerView title="Forma de pagamento" onBack={() => setPicking(false)}>
        <OptionList options={options} value={methodId ?? 0} onPick={async (id) => {
          const next = id || null;
          setPicking(false);
          if (await run(() => save({ paymentMethodId: next }))) setMethodId(next);
        }} />
      </PickerView>
    );
  }

  return (
    <div>
      <div className="sheet-head">
        <h2 className="sheet-title">{account.name}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <ListGroup>
        <ListRow label="Forma de pagamento" value={method?.name ?? "Nenhuma"} onClick={() => setPicking(true)} chevron />
        <ListRow label="Conta ativa" trailing={<Switch checked={enabled} disabled={busy} label="Conta ativa" onChange={async (v) => { if (await run(() => save({ enabled: v }))) setEnabled(v); }} />} />
      </ListGroup>
      {enabled && <ImportForm account={account} />}
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions">
        {confirm
          ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={(e) => e.detail < 2 && void run(() => api.del(path)).then((ok) => ok && onClose())}>Confirmar exclusão</button>
          : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirm(true)}>Excluir conta</button>}
      </div>
    </div>
  );
}

function ImportForm({ account }: { account: BankAccount }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [format, setFormat] = useState<"ofx" | "csv">("ofx");
  const [file, setFile] = useState<File | null>(null);
  const [cols, setCols] = useState({ dateColumn: "1", descriptionColumn: "2", amountColumn: "3" });
  const [hasHeader, setHasHeader] = useState(true);
  const [invertSign, setInvertSign] = useState(false);
  const [result, setResult] = useState("");

  async function send(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    const form = new FormData();
    form.set("format", format);
    form.set("file", file);
    if (format === "csv") {
      for (const [k, v] of Object.entries(cols)) form.set(k, v);
      form.set("hasHeader", String(hasHeader));
      form.set("invertSign", String(invertSign));
    }
    setResult("");
    await run(async () => {
      const r = await api.upload<{ imported: number; duplicates: number }>(`/wallets/${wallet.id}/bank-accounts/${account.id}/import`, form);
      setResult(`${r.imported} ${r.imported === 1 ? "linha nova" : "linhas novas"}, ${r.duplicates} já ${r.duplicates === 1 ? "importada" : "importadas"} antes.`);
    });
  }

  return (
    <form className="import-form" onSubmit={send}>
      <h3 className="sheet-sub">Importar extrato</h3>
      <Segmented<"ofx" | "csv"> label="Formato do extrato" value={format} onChange={setFormat} options={[{ value: "ofx", label: "OFX" }, { value: "csv", label: "CSV" }]} />
      <p className="meta">OFX é o mais confiável. Use CSV se o banco só oferece esse formato.</p>
      <input type="file" className="file-input" aria-label="Arquivo do extrato" accept={format === "ofx" ? ".ofx,.qfx" : ".csv,.txt"} onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
      {format === "csv" && (
        <ListGroup>
          {(["dateColumn", "descriptionColumn", "amountColumn"] as const).map((k) => {
            const label = { dateColumn: "Coluna da data", descriptionColumn: "Coluna da descrição", amountColumn: "Coluna do valor" }[k];
            return (
              <div className="list-row" key={k}>
                <label htmlFor={`col-${k}`} className="lr-label">{label}</label>
                <input id={`col-${k}`} className="desc-input" inputMode="numeric" aria-label={label} value={cols[k]} onChange={(e) => setCols({ ...cols, [k]: e.target.value })} />
              </div>
            );
          })}
          <label className="list-row"><span className="lr-label">A primeira linha é o cabeçalho</span><input type="checkbox" checked={hasHeader} onChange={(e) => setHasHeader(e.target.checked)} /></label>
          <label className="list-row"><span className="lr-label">Compras aparecem como valores positivos (fatura de cartão)</span><input type="checkbox" checked={invertSign} onChange={(e) => setInvertSign(e.target.checked)} /></label>
        </ListGroup>
      )}
      {error && <p className="error" role="alert">{error}</p>}
      {result && <p role="status">{result}</p>}
      <div className="actions"><button type="submit" className="btn full" disabled={busy || !file}>Importar</button></div>
    </form>
  );
}
