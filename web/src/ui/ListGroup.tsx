import type { ReactNode } from "react";
import { Link } from "react-router";

/** A white block of rows, as in system settings lists. */
export function ListGroup({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`list-group ${className}`.trim()}>{children}</div>;
}

/**
 * One row. With `onClick` the whole row is a button (label and value form its name); without it, a plain row that can hold
 * `trailing` controls such as a Switch.
 */
export function ListRow({ id, label, value, icon, onClick, to, chevron, trailing, disabled }: {
  id?: string;
  label: ReactNode;
  value?: ReactNode;
  icon?: ReactNode;
  onClick?: () => void;
  /** Makes the row a link to another screen. */
  to?: string;
  chevron?: boolean;
  trailing?: ReactNode;
  disabled?: boolean;
}) {
  const content = (
    <>
      {icon !== undefined && <span className="lr-icon" aria-hidden="true">{icon}</span>}
      <span className="lr-label">{label}</span>
      {value !== undefined && <>{" "}<span className="lr-value">{value}</span></>}
      {trailing}
      {chevron && <span className="lr-chevron" aria-hidden="true">›</span>}
    </>
  );
  if (to) return <Link to={to} className="list-row">{content}</Link>;
  if (!onClick) return <div className="list-row">{content}</div>;
  return (
    <button type="button" id={id} className="list-row" onClick={onClick} disabled={disabled}>
      {content}
    </button>
  );
}
