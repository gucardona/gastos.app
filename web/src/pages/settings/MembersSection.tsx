import { useState } from "react";
import { api } from "../../api/client";
import type { Member } from "../../api/types";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { MemberSheet } from "../../components/MemberSheet";
import { Segmented } from "../../components/Segmented";
import { Sheet } from "../../components/Sheet";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

const ROLE_LABEL = { owner: "Administra", editor: "Pode editar", reader: "Só leitura" } as const;

export function MembersSection() {
  const { user, wallet, members } = useApp();
  const owner = wallet.role === "owner";
  const [open, setOpen] = useState<Member | null>(null);
  const [inviting, setInviting] = useState(false);

  // the owner opens anyone; everyone else only their own row, to leave the wallet
  const actionable = (m: Member) => owner || (m.userId === user.id && m.role !== "owner");

  return (
    <div className="screen">
      <ListGroup>
        {members.map((m) => (
          <ListRow
            key={m.userId}
            label={<><span className="desc">{m.name}{m.userId === user.id && " (você)"}</span><span className="meta">{m.email}</span></>}
            value={ROLE_LABEL[m.role]}
            onClick={actionable(m) ? () => setOpen(m) : undefined}
            chevron={actionable(m)}
          />
        ))}
      </ListGroup>
      {owner && <button type="button" className="btn ghost full" onClick={() => setInviting(true)}>Convidar alguém</button>}
      <MemberSheet member={open} onClose={() => setOpen(null)} />
      <Sheet open={inviting} onClose={() => setInviting(false)} label="Convidar alguém">
        {inviting && <InviteForm onClose={() => setInviting(false)} />}
      </Sheet>
    </div>
  );
}

function InviteForm({ onClose }: { onClose: () => void }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<"editor" | "reader">("editor");
  return (
    <form onSubmit={async (e) => { e.preventDefault(); if (await run(() => api.post(`/wallets/${wallet.id}/members`, { email, role }))) onClose(); }}>
      <div className="sheet-head">
        <h2 className="sheet-title">Convidar alguém</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      <p className="meta">A pessoa precisa já ter uma conta no gastos.</p>
      <div className="field-row">
        <label htmlFor="invite-email" className="lr-label">E-mail</label>
        <input id="invite-email" className="desc-input" type="email" aria-label="E-mail do convidado" value={email} onChange={(e) => setEmail(e.target.value)} />
      </div>
      <Segmented<"editor" | "reader"> label="Permissão do convidado" value={role} onChange={setRole} options={[{ value: "editor", label: "Pode editar" }, { value: "reader", label: "Só leitura" }]} />
      {error && <p className="error" role="alert">{error}</p>}
      <div className="actions"><button type="submit" className="btn full" disabled={busy || !email.trim()}>Convidar</button></div>
    </form>
  );
}
