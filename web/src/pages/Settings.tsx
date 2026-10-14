import { useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";
import { Link, Navigate, useNavigate, useParams, useSearchParams } from "react-router";
import { api } from "../api/client";
import { ListGroup } from "../components/ListGroup";
import { PageBar } from "../components/PageBar";
import { RecurringSection } from "../components/RecurringSection";
import { Segmented } from "../components/Segmented";
import { useApp } from "../context";
import { setTheme, storedTheme, type Theme } from "../lib/theme";
import { BankSection } from "./settings/BankSection";
import { BudgetsSection } from "./settings/BudgetsSection";
import { CategoriesSection } from "./settings/CategoriesSection";
import { MembersSection } from "./settings/MembersSection";
import { PaymentsSection } from "./settings/PaymentsSection";
import { WalletSection } from "./settings/WalletSection";

/** Subjects of Ajustes, in the order of the index. `group` 0 has no title. */
export const SETTINGS_SECTIONS = [
  { id: "perfil", label: "Perfil", group: 0 },
  { id: "carteira", label: "Carteira", group: 1 },
  { id: "pessoas", label: "Pessoas", group: 1 },
  { id: "categorias", label: "Categorias", group: 1 },
  { id: "pagamentos", label: "Formas de pagamento", group: 1 },
  { id: "orcamentos", label: "Orçamentos", group: 1 },
  { id: "bancos", label: "Contas bancárias", group: 2 },
  { id: "recorrentes", label: "Recorrentes", group: 2 },
  { id: "aparencia", label: "Aparência", group: 2 },
] as const;

const GROUP_TITLES = ["", "Casa", "Dados"];
/** Ids used by the old ?s= addresses that were renamed. */
const OLD_IDS: Record<string, string> = { conta: "perfil", membros: "pessoas" };

export function Settings() {
  const { user, wallet } = useApp();
  const { section } = useParams();
  const [params] = useSearchParams();
  const old = params.get("s");
  if (old) return <Navigate to={`/ajustes/${OLD_IDS[old] ?? old}`} replace />;
  const current = SETTINGS_SECTIONS.find((s) => s.id === section);
  if (section && !current) return <Navigate to="/ajustes" replace />;

  return (
    <>
      <PageBar />
      <div className={`settings-layout ${current ? "has-section" : ""}`}>
        <nav className="settings-index" aria-label="Ajustes">
          {current ? <h2 className="page-title">Ajustes</h2> : <h1 className="page-title">Ajustes</h1>}
          {GROUP_TITLES.map((title, group) => (
            <section key={title || "perfil"}>
              {title && <h2 className="group-title">{title}</h2>}
              <ListGroup>
                {SETTINGS_SECTIONS.filter((s) => s.group === group).map((s) => (
                  <LinkRow key={s.id} to={`/ajustes/${s.id}`} label={s.label} value={s.id === "perfil" ? user.name : undefined} current={s.id === current?.id} />
                ))}
              </ListGroup>
            </section>
          ))}
          <SignOut />
        </nav>
        {!current && (
          // on a wide screen the right side shows the profile until a subject is chosen; on a phone it stays hidden
          <div className="settings-detail default">
            <h2 className="page-title">Perfil</h2>
            <Profile />
          </div>
        )}
        {current && (
          // a key resets the forms when the wallet changes
          <div className="settings-detail" key={`${wallet.id}-${current.id}`}>
            <Link className="back" to="/ajustes" aria-label="Voltar para Ajustes"><span aria-hidden="true">‹</span> Ajustes</Link>
            <h1 className="page-title">{current.label}</h1>
            <Screen id={current.id} />
          </div>
        )}
      </div>
    </>
  );
}

function LinkRow({ to, label, value, current }: { to: string; label: string; value?: ReactNode; current: boolean }) {
  return (
    <Link to={to} className="list-row" aria-current={current ? "page" : undefined}>
      <span className="lr-label">{label}</span>
      {value !== undefined && <>{" "}<span className="lr-value">{value}</span></>}
      <span className="lr-chevron" aria-hidden="true">›</span>
    </Link>
  );
}

function Screen({ id }: { id: (typeof SETTINGS_SECTIONS)[number]["id"] }) {
  switch (id) {
    case "perfil": return <Profile />;
    case "carteira": return <WalletSection />;
    case "pessoas": return <MembersSection />;
    case "categorias": return <CategoriesSection />;
    case "pagamentos": return <PaymentsSection />;
    case "orcamentos": return <BudgetsSection />;
    case "bancos": return <BankSection />;
    case "recorrentes": return <RecurringSection />;
    case "aparencia": return <Appearance />;
  }
}

const ROLE_TEXT = { owner: "você administra", editor: "você pode editar", reader: "só leitura" } as const;

function Profile() {
  const { user, wallet } = useApp();
  return (
    <ListGroup>
      <div className="list-row"><span className="lr-label"><span className="desc">{user.name}</span><span className="meta">{user.email}</span></span></div>
      <div className="list-row"><span className="lr-label">Carteira ativa</span> <span className="lr-value">{wallet.name}, {ROLE_TEXT[wallet.role]}</span></div>
    </ListGroup>
  );
}

function Appearance() {
  const [theme, setThemeState] = useState<Theme>(storedTheme);
  return (
    <Segmented<Theme> label="Tema" value={theme} onChange={(t) => { setThemeState(t); setTheme(t); }}
      options={[{ value: "system", label: "Automático" }, { value: "light", label: "Claro" }, { value: "dark", label: "Escuro" }]} />
  );
}

function SignOut() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  async function logout() {
    await api.post("/auth/logout").catch(() => undefined);
    qc.clear();
    navigate("/login", { replace: true });
  }
  return (
    <ListGroup className="sign-out">
      <button type="button" className="list-row danger-row" onClick={() => void logout()}>Sair</button>
    </ListGroup>
  );
}
