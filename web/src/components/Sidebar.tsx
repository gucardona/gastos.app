import { Link, NavLink } from "react-router";
import { useApp } from "../context";
import { Icon } from "./Icon";
import { formatBadge, navItems, useMonthSearch } from "./nav";
import { WalletSwitcher } from "./WalletSwitcher";

/** Desktop navigation: wallet on top, destinations, the new-transaction button and the profile, which leads to Ajustes. */
export function Sidebar({ pending, onAdd }: { pending: number; onAdd: () => void }) {
  const { user, wallet, canEdit, assistantEnabled } = useApp();
  const badge = formatBadge(pending);
  const search = useMonthSearch();
  return (
    <aside className="sidebar" aria-label="Barra lateral">
      <WalletSwitcher />
      <nav aria-label="Navegação">
        {navItems(wallet.splittingEnabled, assistantEnabled).map((item) => (
          <NavLink key={item.to} to={{ pathname: item.to, search }} end={item.end}>
            <Icon name={item.icon} />
            {item.label}
            {item.to === "/caixa" && badge && <b className="badge">{badge}</b>}
          </NavLink>
        ))}
      </nav>
      <div className="grow" />
      {canEdit && (
        <button type="button" className="btn add" onClick={onAdd}>
          <Icon name="plus" />
          Novo lançamento
        </button>
      )}
      <Link className="profile" to="/ajustes" aria-label="Ajustes">
        <span className="avatar">{user.name[0]?.toUpperCase()}</span>
        <span>{user.name}<small>Ajustes</small></span>
      </Link>
    </aside>
  );
}
