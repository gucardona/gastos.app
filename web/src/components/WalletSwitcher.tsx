import { useState } from "react";
import { firstName, useApp } from "../context";
import { OptionList } from "./pickers";
import { Sheet } from "./Sheet";

/** The active wallet and who shares it; with more than one wallet, tapping it opens the list to switch. */
export function WalletSwitcher() {
  const { user, wallet, wallets, setWalletId, members } = useApp();
  const [open, setOpen] = useState(false);
  const others = members.filter((m) => m.userId !== user.id).map((m) => firstName(m.name));
  const shared = others.length > 0 && <small>com {others.join(", ")}</small>;

  if (wallets.length < 2) {
    return <div className="wallet"><strong className="wallet-name">{wallet.name}</strong>{shared}</div>;
  }
  return (
    <div className="wallet">
      <button type="button" className="wallet-btn" aria-label={`Carteira: ${wallet.name}`} aria-haspopup="dialog" onClick={() => setOpen(true)}>
        <span className="wallet-name">{wallet.name}</span>
        <span className="wallet-caret" aria-hidden="true">▾</span>
      </button>
      {shared}
      <Sheet open={open} onClose={() => setOpen(false)} label="Trocar de carteira">
        <div className="sheet-head">
          <h2 className="sheet-title">Carteiras</h2>
          <button type="button" className="icon-btn" onClick={() => setOpen(false)} aria-label="Fechar">✕</button>
        </div>
        <OptionList options={wallets.map((w) => ({ value: w.id, label: w.role === "owner" ? w.name : `${w.name}, de ${firstName(w.ownerName)}` }))} value={wallet.id} onPick={(id) => { setWalletId(id); setOpen(false); }} />
      </Sheet>
    </div>
  );
}
