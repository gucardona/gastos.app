import { Link, NavLink } from "react-router";
import { firstName, useApp } from "../context";
import { Icon } from "./Icon";
import { formatBadge, navItems } from "./nav";

/** Desktop navigation: wallet on top, destinations, the new-transaction button and the profile, which leads to Ajustes. */
export function Sidebar({ pending, onAdd }: { pending: number; onAdd: () => void }) {
  const { user, wallet, wallets, setWalletId, members, canEdit } = useApp();
  const others = members.filter((m) => m.userId !== user.id).map((m) => firstName(m.name));
  const badge = formatBadge(pending);
  return (
    <aside className="sidebar" aria-label="Barra lateral">
      <label className="wallet">
        <select aria-label="Carteira" value={wallet.id} onChange={(e) => setWalletId(Number(e.target.value))}>
          {wallets.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}
        </select>
        {others.length > 0 && <small>com {others.join(", ")}</small>}
      </label>
      <nav aria-label="Navegação">
        {navItems(wallet.splittingEnabled).map((item) => (
          <NavLink key={item.to} to={item.to} end={item.end}>
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
