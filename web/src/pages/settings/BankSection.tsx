import { useState } from "react";
import { api } from "../../api/client";
import { useBankAccounts, useRules } from "../../api/queries";
import type { BankAccount } from "../../api/types";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

export function BankSection() {
  const { wallet, canEdit } = useApp();
  const accounts = useBankAccounts(wallet.id);
  return (
    <div className="settings">
      <p className="muted" style={{ margin: "0 0 8px" }}>Baixe o extrato (OFX ou CSV) no app ou site do seu banco e envie aqui. Nada é lançado sem a sua confirmação na Caixa de entrada.</p>
      {accounts.error && <p className="error" role="alert">Não foi possível carregar as contas.</p>}
      {accounts.data?.map((a) => <Account key={a.id} a={a} />)}
      {canEdit && <AddAccount />}
      <Rules />
    </div>
  );
}

function AddAccount() {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  if (!open) return <button type="button" className="btn ghost" style={{ justifySelf: "start", marginTop: 12 }} onClick={() => setOpen(true)}>Nova conta bancária</button>;
  return (
    <form className="item inline-form" onSubmit={async (e) => { e.preventDefault(); if (await run(() => api.post(`/wallets/${wallet.id}/bank-accounts`, { name }))) { setOpen(false); setName(""); } }}>
      <label>Nome<input className="text" aria-label="Nome da conta bancária" value={name} onChange={(e) => setName(e.target.value)} /></label>
      {error && <p className="error" role="alert">{error}</p>}
      <div className="item-actions"><button type="submit" className="btn" disabled={busy}>Criar</button><button type="button" className="btn ghost" onClick={() => setOpen(false)}>Cancelar</button></div>
    </form>
  );
}

function Account({ a }: { a: BankAccount }) {
  const { wallet, paymentMethods, canEdit } = useApp();
  const { run, busy, error } = useAction();
  const [importing, setImporting] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const path = `/wallets/${wallet.id}/bank-accounts/${a.id}`;
  const save = (over: Partial<BankAccount>) => api.put(path, { name: a.name, paymentMethodId: a.paymentMethodId, enabled: a.enabled, ...over });

  return (
    <div className="item" style={a.enabled ? undefined : { opacity: 0.65 }}>
      <div className="line" style={{ borderTop: 0, padding: 0 }}>
        <div>{a.name}<small>{a.enabled ? (a.pendingCount ? `${a.pendingCount} para revisar` : "em dia") : "desativada"}</small></div>
      </div>
      {canEdit && (
        <>
          <div className="inline-form" style={{ marginTop: 8 }}>
            <label>Forma de pagamento dos lançamentos
              <select aria-label={`Forma de pagamento de ${a.name}`} value={a.paymentMethodId ?? 0} disabled={busy}
                onChange={(e) => void run(() => save({ paymentMethodId: Number(e.target.value) || null }))}>
                <option value={0}>Nenhuma</option>
                {paymentMethods.filter((p) => !p.archived || p.id === a.paymentMethodId).map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            </label>
          </div>
          <div className="item-actions">
            {a.enabled && <button type="button" className="chip" aria-expanded={importing} onClick={() => setImporting(!importing)}>Importar extrato</button>}
            <button type="button" className="chip" disabled={busy} onClick={() => void run(() => save({ enabled: !a.enabled }))}>{a.enabled ? "Desativar" : "Reativar"}</button>
            {confirm
              ? <button type="button" className="chip danger" disabled={busy} onClick={() => void run(() => api.del(path))}>Confirmar exclusão</button>
              : <button type="button" className="chip" onClick={() => setConfirm(true)}>Excluir</button>}
          </div>
          {importing && <ImportForm a={a} />}
        </>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}

function ImportForm({ a }: { a: BankAccount }) {
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
      const r = await api.upload<{ imported: number; duplicates: number }>(`/wallets/${wallet.id}/bank-accounts/${a.id}/import`, form);
      setResult(`${r.imported} ${r.imported === 1 ? "linha nova" : "linhas novas"}, ${r.duplicates} já ${r.duplicates === 1 ? "importada" : "importadas"} antes.`);
    });
  }

  return (
    <form className="inline-form" style={{ marginTop: 10 }} onSubmit={send}>
      <label>Formato
        <select aria-label="Formato do extrato" value={format} onChange={(e) => setFormat(e.target.value as "ofx" | "csv")}>
          <option value="ofx">OFX (recomendado)</option>
          <option value="csv">CSV</option>
        </select>
      </label>
      <label>Arquivo<input type="file" aria-label="Arquivo do extrato" accept={format === "ofx" ? ".ofx,.qfx" : ".csv,.txt"} onChange={(e) => setFile(e.target.files?.[0] ?? null)} /></label>
      {format === "csv" && (
        <>
          <label>Coluna da data<input className="text" inputMode="numeric" aria-label="Coluna da data" value={cols.dateColumn} onChange={(e) => setCols({ ...cols, dateColumn: e.target.value })} /></label>
          <label>Coluna da descrição<input className="text" inputMode="numeric" aria-label="Coluna da descrição" value={cols.descriptionColumn} onChange={(e) => setCols({ ...cols, descriptionColumn: e.target.value })} /></label>
          <label>Coluna do valor<input className="text" inputMode="numeric" aria-label="Coluna do valor" value={cols.amountColumn} onChange={(e) => setCols({ ...cols, amountColumn: e.target.value })} /></label>
          <label className="inline"><input type="checkbox" checked={hasHeader} onChange={(e) => setHasHeader(e.target.checked)} /> A primeira linha é o cabeçalho</label>
          <label className="inline"><input type="checkbox" checked={invertSign} onChange={(e) => setInvertSign(e.target.checked)} /> Compras aparecem como valores positivos (fatura de cartão)</label>
        </>
      )}
      {error && <p className="error" role="alert">{error}</p>}
      {result && <p role="status">{result}</p>}
      <div className="item-actions"><button type="submit" className="btn" disabled={busy || !file}>Importar</button></div>
    </form>
  );
}

function Rules() {
  const { wallet, categories, canEdit } = useApp();
  const rules = useRules(wallet.id);
  const { run, busy, error } = useAction();
  if (!rules.data?.length) return null;
  return (
    <>
      <h2 style={{ font: "600 17px var(--display)", margin: "20px 0 4px" }}>Regras de categoria</h2>
      {rules.data.map((r) => {
        const c = categories.find((x) => x.id === r.categoryId);
        return (
          <div className="line" key={r.id}>
            <div>“{r.pattern}”<small>{c ? `${c.icon} ${c.name}` : "categoria removida"}{r.renameTo ? `, renomeia para ${r.renameTo}` : ""}</small></div>
            {canEdit && <button type="button" className="chip" disabled={busy} aria-label={`Excluir regra ${r.pattern}`} onClick={() => void run(() => api.del(`/wallets/${wallet.id}/rules/${r.id}`))}>Excluir</button>}
          </div>
        );
      })}
      {error && <p className="error" role="alert">{error}</p>}
    </>
  );
}
