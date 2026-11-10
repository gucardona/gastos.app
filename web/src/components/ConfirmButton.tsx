import { useEffect, useRef, useState } from "react";

/**
 * The app's two-step button: the first click turns it into the question ("Confirmar …"), and only a second, separate click
 * confirms (a double click does not). The question takes the focus; leaving it (focus elsewhere) or Esc cancels.
 */
export function ConfirmButton({ label, confirmLabel, ariaLabel, confirmAriaLabel, className, confirmClassName = className, disabled, onConfirm }: {
  label: string;
  confirmLabel: string;
  ariaLabel?: string;
  confirmAriaLabel?: string;
  className: string;
  confirmClassName?: string;
  disabled?: boolean;
  onConfirm: () => void;
}) {
  const [asking, setAsking] = useState(false);
  const ref = useRef<HTMLButtonElement>(null);
  useEffect(() => { if (asking) ref.current?.focus(); }, [asking]);

  if (!asking) return <button key="ask" type="button" className={className} disabled={disabled} aria-label={ariaLabel} onClick={() => setAsking(true)}>{label}</button>;
  return (
    <button
      key="confirm" ref={ref} type="button" className={confirmClassName} disabled={disabled} aria-label={confirmAriaLabel}
      onBlur={() => setAsking(false)}
      onKeyDown={(e) => e.key === "Escape" && setAsking(false)}
      onClick={(e) => { if (e.detail < 2) { setAsking(false); onConfirm(); } }}
    >
      {confirmLabel}
    </button>
  );
}
