import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";
import { useRefresh } from "../api/queries";
import type { InboxItem } from "../api/types";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { formatBRL } from "../lib/money";
import { ListGroup, ListRow } from "./ListGroup";
import { CategoryGrid, PickerView } from "./pickers";
import { Sheet } from "./Sheet";
import { Switch } from "./Switch";

const shortDate = (iso: string) => new Date(`${iso}T12:00:00`).toLocaleDateString("pt-BR", { day: "numeric", month: "long" });

/** Decide one line of an imported statement: accept it as a transaction, link it to one already typed in, or ignore it. */
export function InboxSheet({ item, onClose }: { item: InboxItem | null; onClose: () => void }) {
  return (
    <Sheet open={item !== null} onClose={onClose} label="Decidir linha do extrato">
      {item && <InboxForm key={item.id} item={item} onClose={onClose} />}
    </Sheet>
  );
}

function InboxForm({ item, onClose }: { item: InboxItem; onClose: () => void }) {
  const { wallet, categories } = useApp();
  const { run, busy, error } = useAction();
  const refresh = useRefresh();
  const kind = item.amountCents > 0 ? "income" : "expense";
  const defaultDescription = item.suggestedDescription || item.description;
  const options = categories.filter((c) => c.kind === kind && (!c.archived || c.id === item.suggestedCategoryId));

  const [categoryId, setCategoryId] = useState(item.suggestedCategoryId ?? 0);
  const [description, setDescription] = useState(defaultDescription);
  const [remember, setRemember] = useState(false);
  const [pattern, setPattern] = useState(() => item.description.toLowerCase().split(/\s+/).slice(0, 2).join(" "));
  const [problem, setProblem] = useState("");
  const [picking, setPicking] = useState(false);
  const [ruleFailed, setRuleFailed] = useState(false); // the line was accepted but the rule was refused
  const cameFromPicker = useRef(false);

  // coming back from the category grid, focus goes to the row that opened it
  useEffect(() => {
    if (!picking && cameFromPicker.current) {
      document.getElementById("row-inbox-category")?.focus();
      cameFromPicker.current = false;
    }
  }, [picking]);

  const base = `/wallets/${wallet.id}/inbox/${item.id}`;
  const category = categories.find((c) => c.id === categoryId);
  const offerRule = !item.suggestedCategoryId && categoryId > 0;

  /** Runs the call, closes on success; on failure the list may be stale (someone else decided first), so refresh it anyway. */
  async function decide(call: () => Promise<unknown>) {
    if (await run(call)) onClose();
    else refresh(wallet.id);
  }

  async function accept() {
    const length = [...pattern.trim()].length;
    if (offerRule && remember && length < 2) return setProblem("Use ao menos 2 caracteres na regra.");
    if (offerRule && remember && length > 60) return setProblem("Use no máximo 60 caracteres na regra.");
    setProblem("");
    const body: { categoryId: number; description?: string } = { categoryId };
    const text = description.trim();
    if (text && text !== defaultDescription) body.description = text;
    if (!(await run(() => api.post(`${base}/accept`, body)))) return refresh(wallet.id); // maybe someone else decided first: the list is stale
    // the rule is a second step: if it fails the line is already a transaction, and accepting again would only give a 409
    if (offerRule && remember && !(await run(() => api.post(`/wallets/${wallet.id}/rules`, { pattern: pattern.trim(), categoryId })))) return setRuleFailed(true);
    onClose();
  }

  if (picking) {
    return (
      <PickerView title="Categoria" onBack={() => setPicking(false)}>
        <CategoryGrid categories={options} value={categoryId || null} onPick={(id) => { setCategoryId(id); setPicking(false); }} />
      </PickerView>
    );
  }

  if (ruleFailed) {
    return (
      <div>
        <h2 className="sheet-title">Lançamento criado, mas a regra não foi salva.</h2>
        {error && <p className="error" role="alert">{error}</p>}
        <div className="actions"><button type="button" className="btn full" onClick={onClose}>Fechar</button></div>
      </div>
    );
  }

  return (
    <div>
      <div className="sheet-head">
        <h2 className="sheet-title">{item.suggestedDescription || item.description}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <p className="meta">{kind === "income" ? "+" : "−"}{formatBRL(Math.abs(item.amountCents))}, {shortDate(item.date)}, {item.accountName}</p>
      {item.suggestedMatch && (
        <p className="meta">Parece o lançamento que você já fez: {item.suggestedMatch.description}, {shortDate(item.suggestedMatch.date)}.</p>
      )}

      <ListGroup>
        <ListRow id="row-inbox-category" label="Categoria" value={category ? `${category.icon} ${category.name}` : "Escolha"} onClick={() => { cameFromPicker.current = true; setPicking(true); }} chevron />
        <div className="list-row">
          <label htmlFor="inbox-desc" className="lr-label">Descrição</label>
          <input id="inbox-desc" className="desc-input" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        {offerRule && (
          <ListRow label="Usar nas próximas importações" trailing={<Switch checked={remember} onChange={(v) => { setRemember(v); setProblem(""); }} label="Usar nas próximas importações" />} />
        )}
        {offerRule && remember && (
          <div className="list-row">
            <label htmlFor="inbox-rule" className="lr-label">Texto da regra</label>
            <input id="inbox-rule" className="desc-input" value={pattern} onChange={(e) => { setPattern(e.target.value); setProblem(""); }} />
          </div>
        )}
      </ListGroup>

      {(problem || error) && <p className="error" role="alert">{problem || error}</p>}
      <div className="actions">
        <button type="button" className="btn full" disabled={busy || categoryId === 0} onClick={() => void accept()}>Aceitar</button>
        {item.suggestedMatch && (
          <button type="button" className="btn ghost full" disabled={busy} onClick={() => void decide(() => api.post(`${base}/match`, { transactionId: item.suggestedMatch!.id }))}>É este lançamento</button>
        )}
        <button type="button" className="btn ghost full" disabled={busy} onClick={() => void decide(() => api.post(`${base}/ignore`))}>Ignorar</button>
      </div>
    </div>
  );
}
