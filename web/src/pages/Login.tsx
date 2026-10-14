import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router";
import { ApiError, api } from "../api/client";

export function Login() {
  const [register, setRegister] = useState(false);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [inviteCode, setInviteCode] = useState("");
  // Who may sign up is decided by the server: open (first account), invite code, or closed.
  const registration = useQuery({ queryKey: ["registration"], queryFn: () => api.get<{ mode: "open" | "code" | "closed" }>("/auth/registration"), retry: false });
  const mode = registration.data?.mode;
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [busy, setBusy] = useState(false);
  const qc = useQueryClient();
  const navigate = useNavigate();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setGeneral("");
    if (register && password !== confirm) {
      setErrors({ confirm: "As senhas não são iguais." });
      return;
    }
    setBusy(true);
    try {
      await api.post(register ? "/auth/register" : "/auth/login", register ? { name, email, password, ...(mode === "code" ? { inviteCode } : {}) } : { email, password });
      qc.clear();
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setErrors(err.fields);
        setGeneral(Object.keys(err.fields).length ? "" : err.message);
      } else setGeneral("Sem conexão. Tente de novo.");
    } finally {
      setBusy(false);
    }
  }

  const error = (k: string) => errors[k] && <p className="error" role="alert">{errors[k]}</p>;
  /** One labelled row; the error of the field sits right under it. */
  const row = (id: string, label: string, input: React.ReactNode, key?: string) => (
    <div className="auth-field">
      <div className="field-row"><label htmlFor={id} className="lr-label">{label}</label>{input}</div>
      {key && error(key)}
    </div>
  );

  return (
    <div className="auth">
      <form onSubmit={submit}>
        <h1>gastos</h1>
        {register && row("name", "Como você gostaria de ser chamado?", <input id="name" className="desc-input" autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} />, "name")}
        {row("email", "E-mail", <input id="email" className="desc-input" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />, "email")}
        {row("password", "Senha", <input id="password" className="desc-input" type="password" autoComplete={register ? "new-password" : "current-password"} value={password} onChange={(e) => setPassword(e.target.value)} />, "password")}
        {register && row("confirm", "Repita a senha", <input id="confirm" className="desc-input" type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />, "confirm")}
        {register && mode === "code" && row("invite", "Código de convite", <input id="invite" className="desc-input" autoComplete="off" value={inviteCode} onChange={(e) => setInviteCode(e.target.value)} />)}
        {general && <p className="error" role="alert">{general}</p>}
        <button type="submit" className="btn full" disabled={busy}>{register ? "Criar conta" : "Entrar"}</button>
        {/* the toggle waits for the mode so a closed sign-up never flashes an option that cannot work */}
        {mode === "closed" && <p className="muted lead">O cadastro está fechado. Peça um convite.</p>}
        {!registration.isPending && mode !== "closed" && (
          <button type="button" className="switch-mode" onClick={() => { setRegister(!register); setErrors({}); setGeneral(""); }}>
            {register ? "Já tenho conta" : "Criar uma conta"}
          </button>
        )}
      </form>
    </div>
  );
}
