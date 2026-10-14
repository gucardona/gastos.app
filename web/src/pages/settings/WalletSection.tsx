import { useState } from "react";
import { api } from "../../api/client";
import type { Wallet } from "../../api/types";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { Sheet } from "../../components/Sheet";
import { Switch } from "../../components/Switch";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

export function WalletSection() {
  const { wallet, wallets } = useApp();
  const owner = wallet.role === "owner";
  const rename = useAction();
  const toggle = useAction();
  const remove = useAction();
  const [name, setName] = useState(wallet.name);
  const [creating, setCreating] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  return (
    <div className="screen">
      <ListGroup>
        <form className="list-row" onSubmit={(e) => { e.preventDefault(); void rename.run(() => api.patch(`/wallets/${wallet.id}`, { name })); }}>
          <label htmlFor="wname" className="lr-label">Nome da carteira</label>
          <input id="wname" className="desc-input" value={name} disabled={!owner} onChange={(e) => setName(e.target.value)} />
          {owner && name.trim() !== wallet.name && <button type="submit" className="chip" disabled={rename.busy}>Salvar nome</button>}
        </form>
        <ListRow
          label="Dividir gastos entre os membros"
          trailing={<Switch checked={wallet.splittingEnabled} disabled={!owner || toggle.busy} label="Dividir gastos entre os membros" onChange={(v) => void toggle.run(() => api.patch(`/wallets/${wallet.id}`, { splittingEnabled: v }))} />}
        />
      </ListGroup>
      <p className="muted lead">Mostra quem pagou, a divisão e o acerto entre vocês.</p>
      {(rename.error || toggle.error) && <p className="error" role="alert">{rename.error || toggle.error}</p>}

      <button type="button" className="btn ghost full" onClick={() => setCreating(true)}>Nova carteira</button>
      <Sheet open={creating} onClose={() => setCreating(false)} label="Nova carteira">
        {creating && <NewWallet onClose={() => setCreating(false)} />}
      </Sheet>

      {owner && (
        <>
          <ListGroup className="sign-out">
            {confirmDelete
              ? <button key="confirm" type="button" className="list-row danger-row" disabled={remove.busy} onClick={() => void remove.run(() => api.del(`/wallets/${wallet.id}`))}>Confirmar exclusão</button>
              : <button key="ask" type="button" className="list-row danger-row" onClick={() => setConfirmDelete(true)}>Excluir esta carteira</button>}
          </ListGroup>
          <p className="muted lead">Apaga lançamentos, categorias e orçamentos para todos os membros. Não dá para desfazer.{wallets.length === 1 && " É a sua única carteira."}</p>
          {remove.error && <p className="error" role="alert">{remove.error}</p>}
        </>
      )}
    </div>
  );
}

function NewWallet({ onClose }: { onClose: () => void }) {
  const { setWalletId } = useApp();
  const { run, busy, error } = useAction();
  const [name, setName] = useState("");
  const [splitting, setSplitting] = useState(false);

  return (
    <form onSubmit={async (e) => {
      e.preventDefault();
      let created: Wallet | undefined;
      const ok = await run(async () => { created = await api.post<Wallet>("/wallets", { name, splittingEnabled: splitting }); });
      if (ok && created) { setWalletId(created.id); onClose(); }
    }}>
      <div className="sheet-head">
        <h2 className="sheet-title">Nova carteira</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <div className="field-row">
        <label htmlFor="nw-name" className="lr-label">Nome</label>
        <input id="nw-name" className="desc-input" aria-label="Nome da nova carteira" value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      <ListGroup>
        <ListRow label="Dividir gastos entre os membros" trailing={<Switch checked={splitting} label="Dividir gastos na nova carteira" onChange={setSplitting} />} />
      </ListGroup>
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions"><button type="submit" className="btn full" disabled={busy || !name.trim()}>Criar carteira</button></div>
    </form>
  );
}
