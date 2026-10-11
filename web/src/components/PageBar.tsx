import { Link } from "react-router";
import { firstName, useApp } from "../context";
import { addMonths, monthLabel } from "../lib/dates";

/** Top of the screens: active wallet and avatar. */
export function PageBar() {
  const { user, wallet, wallets, setWalletId, members } = useApp();
  const others = members.filter((m) => m.userId !== user.id).map((m) => firstName(m.name));
  return (
    <div className="bar">
      <label className="wallet">
        <select aria-label="Carteira" value={wallet.id} onChange={(e) => setWalletId(Number(e.target.value))}>
          {wallets.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}
        </select>
        {others.length > 0 && <small>com {others.join(", ")}</small>}
      </label>
      <Link className="avatar" to="/ajustes" aria-label="Ajustes">{user.name[0]?.toUpperCase()}</Link>
    </div>
  );
}

export function MonthSwitcher() {
  const { month, setMonth } = useApp();
  return (
    <div className="month">
      <button aria-label="Mês anterior" onClick={() => setMonth(addMonths(month, -1))}>‹</button>
      <span>{monthLabel(month)}</span>
      <button aria-label="Próximo mês" onClick={() => setMonth(addMonths(month, 1))}>›</button>
    </div>
  );
}
