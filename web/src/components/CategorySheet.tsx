import { useState } from "react";
import { api } from "../api/client";
import type { Category, Kind } from "../api/types";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { ColorSwatches } from "./ColorSwatches";
import { EmojiPicker } from "./EmojiPicker";
import { Segmented } from "./Segmented";
import { Sheet } from "./Sheet";

const ESSENTIALITY = [
  { value: "essential", label: "Essencial" },
  { value: "nonessential", label: "Supérfluo" },
  { value: "investment", label: "Investimento" },
];

/** Create (`"new"`, of the given `kind`) or edit a category; closed while `category` is null. */
export function CategorySheet({ category, kind, onClose }: { category: Category | "new" | null; kind: Kind; onClose: () => void }) {
  return (
    <Sheet open={category !== null} onClose={onClose} label={category === "new" ? "Nova categoria" : "Editar categoria"}>
      {category && <CategoryForm key={category === "new" ? "new" : category.id} category={category} kind={kind} onClose={onClose} />}
    </Sheet>
  );
}

function CategoryForm({ category, kind, onClose }: { category: Category | "new"; kind: Kind; onClose: () => void }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const c = category === "new" ? null : category;
  const k = c?.kind ?? kind;
  const [name, setName] = useState(c?.name ?? "");
  const [icon, setIcon] = useState(c?.icon ?? "◈");
  const [color, setColor] = useState(c?.color ?? "#0E6577");
  const [essentiality, setEssentiality] = useState<string>(c?.essentiality ?? "essential");
  const [confirm, setConfirm] = useState(false);
  const [archivedNote, setArchivedNote] = useState(false);
  const path = c ? `/wallets/${wallet.id}/categories/${c.id}` : "";

  const body = (archived = c?.archived ?? false) => ({
    kind: k, name, icon, color, essentiality: k === "expense" ? essentiality : null, sortOrder: c?.sortOrder ?? 0, archived,
  });

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const ok = c
      ? await run(() => api.put(path, body()))
      : await run(() => api.post(`/wallets/${wallet.id}/categories`, { kind: k, name, icon, color, ...(k === "expense" ? { essentiality } : {}) }));
    if (ok) onClose();
  }

  async function remove() {
    let archivedNow = false;
    if (!(await run(async () => { archivedNow = (await api.del(path)) !== undefined; }))) return;
    if (archivedNow) setArchivedNote(true); // it had transactions, so it was archived instead
    else onClose();
  }

  if (archivedNote) {
    return (
      <div>
        <h2 className="sheet-title">Arquivada, porque já tem lançamentos.</h2>
        <div className="actions"><button type="button" className="btn full" onClick={onClose}>Fechar</button></div>
      </div>
    );
  }

  return (
    <form onSubmit={save}>
      <div className="sheet-head">
        <h2 className="sheet-title">{c ? "Editar categoria" : "Nova categoria"}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <div className="preview" data-testid="category-preview">
        <span className="preview-icon" style={{ background: `color-mix(in srgb, ${color} 18%, transparent)` }} aria-hidden="true">{icon}</span>
        <strong>{name || "Nova categoria"}</strong>
      </div>
      <div className="field-row">
        <label htmlFor="cat-name" className="lr-label">Nome</label>
        <input id="cat-name" className="desc-input" aria-label="Nome da categoria" value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      {k === "expense" && <Segmented label="Tipo de gasto" value={essentiality} onChange={setEssentiality} options={ESSENTIALITY} />}
      <h3 className="sheet-sub">Cor</h3>
      <ColorSwatches value={color} onPick={setColor} />
      <h3 className="sheet-sub">Ícone</h3>
      <EmojiPicker value={icon} onPick={setIcon} />
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>{c ? "Salvar" : "Criar categoria"}</button>
        {c?.archived && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void run(() => api.put(path, body(false))).then((ok) => ok && onClose())}>Restaurar</button>}
        {c && !c.archived && <button type="button" className="btn ghost full" disabled={busy} onClick={() => void run(() => api.put(path, body(true))).then((ok) => ok && onClose())}>Arquivar</button>}
        {c && !c.archived && (confirm
          ? <button type="button" className="btn danger full" disabled={busy} onClick={() => void remove()}>Confirmar exclusão</button>
          : <button type="button" className="btn ghost full" onClick={() => setConfirm(true)}>Excluir categoria</button>)}
      </div>
    </form>
  );
}
