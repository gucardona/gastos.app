import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router";
import { ApiError, api } from "../api/client";

export function Login() {
  const [register, setRegister] = useState(false);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [general, setGeneral] = useState("");
  const [busy, setBusy] = useState(false);
  const qc = useQueryClient();
  const navigate = useNavigate();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErrors({});
    setGeneral("");
    try {
      await api.post(register ? "/auth/register" : "/auth/login", register ? { name, email, password } : { email, password });
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

  const field = (k: string) => errors[k] && <p className="error" role="alert">{errors[k]}</p>;

  return (
    <div className="auth">
      <form onSubmit={submit}>
        <h1>gastos</h1>
        {register && (
          <div className="field">
            <label htmlFor="name">Nome</label>
            <input id="name" className="text" autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} />
            {field("name")}
          </div>
        )}
        <div className="field">
          <label htmlFor="email">E-mail</label>
          <input id="email" className="text" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          {field("email")}
        </div>
        <div className="field">
          <label htmlFor="password">Senha</label>
          <input id="password" className="text" type="password" autoComplete={register ? "new-password" : "current-password"} value={password} onChange={(e) => setPassword(e.target.value)} />
          {field("password")}
        </div>
        {general && <p className="error" role="alert">{general}</p>}
        <button type="submit" className="btn" disabled={busy}>{register ? "Criar conta" : "Entrar"}</button>
        <button type="button" className="switch-mode" onClick={() => { setRegister(!register); setErrors({}); setGeneral(""); }}>
          {register ? "Já tenho conta" : "Criar uma conta"}
        </button>
      </form>
    </div>
  );
}
