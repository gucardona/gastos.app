import { Link } from "react-router";
import { useApp } from "../context";
import { addMonths, monthLabel } from "../lib/dates";
import { WalletSwitcher } from "./WalletSwitcher";

/** Top of the screens: active wallet and avatar. */
export function PageBar() {
  const { user } = useApp();
  return (
    <div className="bar">
      <WalletSwitcher />
      <Link className="avatar" to="/ajustes" aria-label="Ajustes">{user.name[0]?.toUpperCase()}</Link>
    </div>
  );
}

export function MonthSwitcher() {
  const { month, setMonth } = useApp();
  return (
    <div className="month">
      <button type="button" aria-label="Mês anterior" onClick={() => setMonth(addMonths(month, -1))}>‹</button>
      <h1 className="month-title">{monthLabel(month)}</h1>
      <button type="button" aria-label="Próximo mês" onClick={() => setMonth(addMonths(month, 1))}>›</button>
    </div>
  );
}
