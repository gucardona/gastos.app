import { useState } from "react";
import { api } from "../api/client";
import { useAction } from "../hooks/useAction";
import { Sheet } from "./Sheet";

/** Change the signed-in user's password; `onChanged` runs after the API accepted it. */
export function PasswordSheet({ open, onClose, onChanged }: { open: boolean; onClose: () => void; onChanged: () => void }) {
  return (
    <Sheet open={open} onClose={onClose} label="Alterar senha">
      {open && <PasswordForm onClose={onClose} onChanged={onChanged} />}
    </Sheet>
  );
}

function PasswordForm({ onClose, onChanged }: { onClose: () => void; onChanged: () => void }) {
  const { run, busy, error } = useAction();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [again, setAgain] = useState("");
  const [problem, setProblem] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (next.length < 8) return setProblem("A nova senha precisa ter 8 ou mais caracteres.");
    if (next !== again) return setProblem("As senhas novas não coincidem.");
    setProblem("");
    if (await run(() => api.put("/me/password", { currentPassword: current, newPassword: next }))) {
      onChanged();
      onClose();
    }
  }

  const field = (id: string, label: string, value: string, set: (v: string) => void, auto: string, focus = false) => (
    <div className="field-row">
      <label htmlFor={id} className="lr-label">{label}</label>
      <input id={id} className="desc-input" type="password" autoComplete={auto} value={value} data-autofocus={focus || undefined} onChange={(e) => set(e.target.value)} />
    </div>
  );

  return (
    <form onSubmit={submit}>
      <div className="sheet-head">
        <h2 className="sheet-title">Alterar senha</h2>
        <button type="button" className="icon-btn" onClick={onClose} aria-label="Fechar">✕</button>
      </div>
      {field("pw-current", "Senha atual", current, setCurrent, "current-password", true)}
      {field("pw-new", "Nova senha", next, setNext, "new-password")}
      {field("pw-again", "Repetir a nova senha", again, setAgain, "new-password")}
      {(problem || error) && <p className="error" role="alert">{problem || error}</p>}
      <p className="muted lead">Ao trocar, as outras sessões desta conta são encerradas.</p>
      <div className="actions">
        <button type="submit" className="btn full" disabled={busy}>Alterar senha</button>
      </div>
    </form>
  );
}
