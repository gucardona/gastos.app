import { useState } from "react";
import { api } from "../../api/client";
import type { Member } from "../../api/types";
import { Switch } from "../../components/Switch";
import { useApp } from "../../context";
import { useAction } from "../../hooks/useAction";

const ROLE_LABEL = { owner: "Administra", editor: "Pode editar", reader: "Só leitura" } as const;

export function MembersSection() {
  const { user, wallet, members } = useApp();
  const owner = wallet.role === "owner";
  const invite = useAction();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<"editor" | "reader">("editor");

  return (
    <div className="settings">
      {members.map((m) => <MemberRow key={m.userId} m={m} isMe={m.userId === user.id} />)}
      {owner && (
        <form className="inline-form item" onSubmit={async (e) => {
          e.preventDefault();
          if (await invite.run(() => api.post(`/wallets/${wallet.id}/members`, { email, role }))) setEmail("");
        }}>
          <strong>Convidar alguém</strong>
          <p className="muted" style={{ margin: 0 }}>A pessoa precisa já ter uma conta no gastos.</p>
          <label>E-mail<input className="text" type="email" aria-label="E-mail do convidado" value={email} onChange={(e) => setEmail(e.target.value)} /></label>
          <label>Permissão
            <select aria-label="Permissão do convidado" value={role} onChange={(e) => setRole(e.target.value as "editor" | "reader")}>
              <option value="editor">Pode editar</option>
              <option value="reader">Só leitura</option>
            </select>
          </label>
          {invite.error && <p className="error" role="alert">{invite.error}</p>}
          <div className="item-actions"><button type="submit" className="btn" disabled={invite.busy || !email.trim()}>Convidar</button></div>
        </form>
      )}
    </div>
  );
}

function MemberRow({ m, isMe }: { m: Member; isMe: boolean }) {
  const { wallet } = useApp();
  const { run, busy, error } = useAction();
  const owner = wallet.role === "owner";
  const [confirm, setConfirm] = useState(false);
  const path = `/wallets/${wallet.id}/members/${m.userId}`;
  const canRemove = m.role !== "owner" && (owner || isMe);

  return (
    <div className="item">
      <div className="line" style={{ borderTop: 0, padding: 0 }}>
        <div>{m.name}{isMe && " (você)"}<small>{m.email}</small></div>
        {owner && m.role !== "owner" ? (
          <select aria-label={`Permissão de ${m.name}`} value={m.role} disabled={busy} onChange={(e) => void run(() => api.patch(path, { role: e.target.value }))}>
            <option value="editor">Pode editar</option>
            <option value="reader">Só leitura</option>
          </select>
        ) : <span className="muted">{ROLE_LABEL[m.role]}</span>}
      </div>
      {owner && wallet.splittingEnabled && (
        <div className="inline" style={{ marginTop: 8 }}>
          <Switch checked={m.splitsByDefault} disabled={busy} label={`Dividir gastos com ${m.name} por padrão`} onChange={(v) => void run(() => api.patch(path, { splitsByDefault: v }))} />
          <span className="muted">Entra na divisão por padrão</span>
        </div>
      )}
      {canRemove && (
        <div className="item-actions">
          {confirm
            ? <button type="button" className="chip danger" disabled={busy} onClick={() => void run(() => api.del(path))}>{isMe ? "Confirmar saída" : "Confirmar remoção"}</button>
            : <button type="button" className="chip" onClick={() => setConfirm(true)}>{isMe ? "Sair da carteira" : "Remover"}</button>}
        </div>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}
