import { NavLink } from "react-router";
import { useApp } from "../context";
import { Icon } from "./Icon";
import { formatBadge, navItems } from "./nav";

/** Phone navigation: a floating bar with the destinations and, apart from it, the + button. */
export function TabBar({ pending, onAdd }: { pending: number; onAdd: () => void }) {
  const { wallet, canEdit, assistantEnabled } = useApp();
  const badge = formatBadge(pending);
  return (
    <nav className="tabbar" aria-label="Navegação">
      <div className="tabbar-pill">
        {navItems(wallet.splittingEnabled, assistantEnabled).map((item) => (
          <NavLink key={item.to} to={item.to} end={item.end}>
            <Icon name={item.icon} />
            {item.label}
            {item.to === "/caixa" && badge && <b className="badge">{badge}</b>}
          </NavLink>
        ))}
      </div>
      {canEdit && (
        <button type="button" className="fab" onClick={onAdd} aria-label="Novo lançamento">
          <Icon name="plus" />
        </button>
      )}
    </nav>
  );
}
