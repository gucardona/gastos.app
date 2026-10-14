import { useState } from "react";
import { api } from "../../api/client";
import { useBankAccounts, useRules } from "../../api/queries";
import type { BankAccount } from "../../api/types";
import { BankAccountSheet } from "../../components/BankAccountSheet";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

export function BankSection() {
  const { wallet, canEdit } = useApp();
  const accounts = useBankAccounts(wallet.id);
  const [open, setOpen] = useState<BankAccount | "new" | null>(null);
  const status = (a: BankAccount) => (!a.enabled ? "desativada" : a.pendingCount ? `${a.pendingCount} para revisar` : "em dia");

  return (
    <div className="screen">
      <p className="muted lead">Baixe o extrato (OFX ou CSV) no app ou site do seu banco e envie aqui. Nada é lançado sem a sua confirmação na Caixa de entrada.</p>
      {accounts.error && <p className="error" role="alert">Não foi possível carregar as contas.</p>}
      {accounts.data && accounts.data.length > 0 && (
        <ListGroup>
          {accounts.data.map((a) => <ListRow key={a.id} label={a.name} value={status(a)} onClick={canEdit ? () => setOpen(a) : undefined} chevron={canEdit} />)}
        </ListGroup>
      )}
      {canEdit && <button type="button" className="btn ghost full" onClick={() => setOpen("new")}>Nova conta bancária</button>}
      <Rules />
      <BankAccountSheet account={open} onClose={() => setOpen(null)} />
    </div>
  );
}

function Rules() {
  const { wallet, categories, canEdit } = useApp();
  const rules = useRules(wallet.id);
  const { run, busy, error } = useAction();
  if (!rules.data?.length) return null;
  return (
    <section>
      <h2 className="group-title">Regras de categoria</h2>
      <ListGroup>
        {rules.data.map((r) => {
          const c = categories.find((x) => x.id === r.categoryId);
          return (
            <div className="list-row" key={r.id}>
              <span className="lr-label">
                <span className="desc">“{r.pattern}”</span>
                <span className="meta">{c ? `${c.icon} ${c.name}` : "categoria removida"}{r.renameTo ? `, renomeia para ${r.renameTo}` : ""}</span>
              </span>
              {canEdit && <button type="button" className="chip" disabled={busy} aria-label={`Excluir regra ${r.pattern}`} onClick={() => void run(() => api.del(`/wallets/${wallet.id}/rules/${r.id}`))}>Excluir</button>}
            </div>
          );
        })}
      </ListGroup>
      {error && <p className="error" role="alert">{error}</p>}
    </section>
  );
}
