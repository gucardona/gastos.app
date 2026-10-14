import { useState } from "react";
import { api } from "../api/client";
import type { Member } from "../api/types";
import { useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { ListGroup, ListRow } from "./ListGroup";
import { Segmented } from "./Segmented";
import { Sheet } from "./Sheet";
import { Switch } from "./Switch";

/** What the person can do with one member: the owner sets permissions and removes; anyone can leave. Closed while `member` is null. */
export function MemberSheet({ member, onClose }: { member: Member | null; onClose: () => void }) {
  return (
    <Sheet open={member !== null} onClose={onClose} label="Pessoa da carteira">
      {member && <MemberForm key={member.userId} member={member} onClose={onClose} />}
    </Sheet>
  );
}

function MemberForm({ member, onClose }: { member: Member; onClose: () => void }) {
  const { user, wallet } = useApp();
  const { run, busy, error } = useAction();
  const [role, setRole] = useState(member.role);
  const [splits, setSplits] = useState(member.splitsByDefault);
  const [confirm, setConfirm] = useState(false);
  const owner = wallet.role === "owner";
  const isMe = member.userId === user.id;
  const canRemove = member.role !== "owner" && (owner || isMe);
  const path = `/wallets/${wallet.id}/members/${member.userId}`;

  return (
    <div>
      <div className="sheet-head">
        <h2 className="sheet-title">{member.name}{isMe && " (você)"}</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <p className="meta">{member.email}</p>
      {owner && member.role !== "owner" && (
        <Segmented<"editor" | "reader">
          label="Permissão"
          value={role as "editor" | "reader"}
          onChange={async (r) => { if (await run(() => api.patch(path, { role: r }))) setRole(r); }}
          options={[{ value: "editor", label: "Pode editar" }, { value: "reader", label: "Só leitura" }]}
        />
      )}
      {owner && wallet.splittingEnabled && (
        <ListGroup>
          <ListRow
            label="Entra na divisão por padrão"
            trailing={<Switch checked={splits} disabled={busy} label={`Dividir gastos com ${member.name} por padrão`} onChange={async (v) => { if (await run(() => api.patch(path, { splitsByDefault: v }))) setSplits(v); }} />}
          />
        </ListGroup>
      )}
      {error && <p className="error" role="alert">{error}</p>}
      {canRemove && (
        <div className="actions">
          {confirm
            ? <button key="confirm" type="button" className="btn danger full" disabled={busy} onClick={() => void run(() => api.del(path)).then((ok) => ok && onClose())}>{isMe ? "Confirmar saída" : "Confirmar remoção"}</button>
            : <button key="ask" type="button" className="btn ghost full" onClick={() => setConfirm(true)}>{isMe ? "Sair da carteira" : "Remover"}</button>}
        </div>
      )}
    </div>
  );
}
