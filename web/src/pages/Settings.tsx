import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { api } from "../api/client";
import { PageBar } from "../components/PageBar";
import { useApp } from "../context";
import { setTheme, storedTheme, type Theme } from "../lib/theme";
import { BankSection } from "./settings/BankSection";
import { BudgetsSection } from "./settings/BudgetsSection";
import { CategoriesSection } from "./settings/CategoriesSection";
import { MembersSection } from "./settings/MembersSection";
import { PaymentsSection } from "./settings/PaymentsSection";
import { WalletSection } from "./settings/WalletSection";

const SECTIONS = [
  { id: "conta", label: "Conta" },
  { id: "carteira", label: "Carteira" },
  { id: "membros", label: "Membros" },
  { id: "categorias", label: "Categorias" },
  { id: "pagamentos", label: "Pagamentos" },
  { id: "orcamentos", label: "Orçamentos" },
  { id: "bancos", label: "Contas bancárias" },
] as const;

export function Settings() {
  const { wallet } = useApp();
  const [params, setParams] = useSearchParams();
  const section = SECTIONS.find((s) => s.id === params.get("s"))?.id ?? "conta";

  return (
    <>
      <PageBar />
      <h1 style={{ font: "700 28px var(--display)", margin: "8px 0 12px" }}>Ajustes</h1>
      <div className="filters" role="group" aria-label="Seções dos ajustes">
        {SECTIONS.map((s) => (
          <button type="button" key={s.id} className="chip" aria-pressed={section === s.id} onClick={() => setParams((p) => { p.set("s", s.id); return p; }, { replace: true })}>{s.label}</button>
        ))}
      </div>
      {/* a key reinicia os formulários ao trocar de carteira */}
      <div key={`${wallet.id}-${section}`}>
        {section === "conta" && <Account />}
        {section === "carteira" && <WalletSection />}
        {section === "membros" && <MembersSection />}
        {section === "categorias" && <CategoriesSection />}
        {section === "pagamentos" && <PaymentsSection />}
        {section === "orcamentos" && <BudgetsSection />}
        {section === "bancos" && <BankSection />}
      </div>
    </>
  );
}

function Account() {
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
      <div className="line"><button type="button" className="btn ghost" onClick={() => void logout()}>Sair</button></div>
    </div>
  );
}
