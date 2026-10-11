import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router";
import { api } from "../api/client";
import { PageBar } from "../components/PageBar";
import { useApp } from "../context";
import { setTheme, storedTheme, type Theme } from "../lib/theme";

export function Settings() {
  const { user, wallet } = useApp();
  const [theme, setThemeState] = useState<Theme>(storedTheme);
  const qc = useQueryClient();
  const navigate = useNavigate();

  async function logout() {
    await api.post("/auth/logout").catch(() => undefined);
    qc.clear();
    navigate("/login", { replace: true });
  }

  return (
    <>
      <PageBar />
      <h1 style={{ font: "700 28px var(--display)", margin: "8px 0 16px" }}>Ajustes</h1>
      <div className="settings">
        <div className="line"><div>{user.name}<small>{user.email}</small></div></div>
        <div className="line"><div>Carteira ativa<small>{wallet.name}, {wallet.role === "owner" ? "você administra" : wallet.role === "editor" ? "você pode editar" : "só leitura"}</small></div></div>
        <div className="line">
          <label htmlFor="theme">Tema</label>
          <select id="theme" value={theme} onChange={(e) => { const t = e.target.value as Theme; setThemeState(t); setTheme(t); }}>
            <option value="system">Do sistema</option>
            <option value="light">Claro</option>
            <option value="dark">Escuro</option>
          </select>
        </div>
        <div className="line"><button className="btn ghost" onClick={() => void logout()}>Sair</button></div>
      </div>
      <p className="muted" style={{ marginTop: 24 }}>Categorias, formas de pagamento, membros e orçamentos chegam na próxima etapa.</p>
    </>
  );
}
