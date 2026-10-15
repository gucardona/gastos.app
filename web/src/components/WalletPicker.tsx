import { useState } from "react";
import type { Wallet } from "../api/types";
import { ListGroup } from "./ListGroup";
import { Sheet } from "./Sheet";

/** Which wallets the assistant looks at: at least one, shown closed as its name or "N carteiras". */
export function WalletPicker({ wallets, selected, onChange }: { wallets: Wallet[]; selected: number[]; onChange: (ids: number[]) => void }) {
  const [open, setOpen] = useState(false);
  const label = selected.length === 1 ? wallets.find((w) => w.id === selected[0])?.name : `${selected.length} carteiras`;
  const toggle = (id: number, on: boolean) => {
    const next = wallets.filter((w) => (w.id === id ? on : selected.includes(w.id))).map((w) => w.id);
    if (next.length > 0) onChange(next);
  };
  return (
    <>
      <button type="button" className="chip wallet-picker" aria-haspopup="dialog" onClick={() => setOpen(true)}>
        {label} <span aria-hidden="true">▾</span>
      </button>
      <Sheet open={open} onClose={() => setOpen(false)} label="Carteiras consultadas">
        <div className="sheet-head">
          <h2 className="sheet-title">Carteiras consultadas</h2>
          <button type="button" className="icon-btn" onClick={() => setOpen(false)} aria-label="Fechar">✕</button>
        </div>
        <ListGroup>
          {wallets.map((w) => {
            const checked = selected.includes(w.id);
            return (
              <label className="list-row" key={w.id}>
                <input type="checkbox" checked={checked} disabled={checked && selected.length === 1} onChange={(e) => toggle(w.id, e.target.checked)} />
                <span className="lr-label">{w.name}</span>
              </label>
            );
          })}
        </ListGroup>
        <div className="actions">
          <button type="button" className="btn ghost" onClick={() => onChange(wallets.map((w) => w.id))}>Selecionar todas</button>
        </div>
      </Sheet>
    </>
  );
}
