import { useState } from "react";
import { api } from "../../api/client";
import type { Wallet } from "../../api/types";
import { Switch } from "../../components/Switch";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

export function WalletSection() {
  const { wallet, wallets, setWalletId } = useApp();
  const owner = wallet.role === "owner";
  const rename = useAction();
  const toggle = useAction();
  const create = useAction();
  const remove = useAction();
  const [name, setName] = useState(wallet.name);
  const [newName, setNewName] = useState("");
  const [newSplitting, setNewSplitting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  return (
    <div className="settings">
      <form className="line" onSubmit={(e) => { e.preventDefault(); void rename.run(() => api.patch(`/wallets/${wallet.id}`, { name })); }}>
        <label htmlFor="wname" style={{ flex: 1 }}>
          Nome da carteira
          <input id="wname" className="text" value={name} disabled={!owner} onChange={(e) => setName(e.target.value)} />
        </label>
        {owner && <button className="chip" disabled={rename.busy || name.trim() === wallet.name}>Salvar nome</button>}
      </form>
      {rename.error && <p className="error" role="alert">{rename.error}</p>}

      <div className="line">
        <div>Dividir gastos entre os membros<small>Mostra quem pagou, a divisão e o acerto entre vocês.</small></div>
        <Switch checked={wallet.splittingEnabled} disabled={!owner || toggle.busy} label="Dividir gastos entre os membros"
          onChange={(v) => void toggle.run(() => api.patch(`/wallets/${wallet.id}`, { splittingEnabled: v }))} />
      </div>
      {toggle.error && <p className="error" role="alert">{toggle.error}</p>}

      <form className="inline-form item" onSubmit={async (e) => {
        e.preventDefault();
        let created: Wallet | undefined;
        const ok = await create.run(async () => { created = await api.post<Wallet>("/wallets", { name: newName, splittingEnabled: newSplitting }); });
        if (ok && created) { setNewName(""); setNewSplitting(false); setWalletId(created.id); }
      }}>
        <strong>Nova carteira</strong>
        <label>Nome<input className="text" aria-label="Nome da nova carteira" value={newName} onChange={(e) => setNewName(e.target.value)} /></label>
        <label className="inline"><Switch checked={newSplitting} label="Dividir gastos na nova carteira" onChange={setNewSplitting} /> <span className="muted">Dividir gastos entre os membros</span></label>
        {create.error && <p className="error" role="alert">{create.error}</p>}
        <div className="item-actions"><button className="btn" disabled={create.busy || !newName.trim()}>Criar carteira</button></div>
      </form>

      {owner && (
        <div className="line">
          <div>Excluir esta carteira<small>Apaga lançamentos, categorias e orçamentos para todos os membros. Não dá para desfazer.{wallets.length === 1 && " É a sua única carteira."}</small></div>
          {confirmDelete
            ? <button className="chip danger" disabled={remove.busy} onClick={() => void remove.run(() => api.del(`/wallets/${wallet.id}`))}>Confirmar exclusão</button>
            : <button className="chip" onClick={() => setConfirmDelete(true)}>Excluir</button>}
        </div>
      )}
      {remove.error && <p className="error" role="alert">{remove.error}</p>}
    </div>
  );
}
