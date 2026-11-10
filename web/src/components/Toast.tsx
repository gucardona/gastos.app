import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";

export type ToastOptions = { action?: { label: string; run: () => void } };
type Toast = { id: number; message: string } & ToastOptions;
type ToastApi = { toast: (message: string, opts?: ToastOptions) => void; current: Toast | null; dismiss: () => void; pause: (on: boolean) => void };

/** Exported for tests, which put a spy in it. */
export const ToastContext = createContext<ToastApi | null>(null);
const noop = () => {};

/** `toast("Lançamento salvo")`, or with `{ action: { label: "Desfazer", run } }`; a no-op outside the provider (tests, Login). */
export const useToast = () => useContext(ToastContext)?.toast ?? noop;

let seq = 0;

/** One short message at a time, after an action; it goes away by itself (4 s, 8 s with an action) unless hovered or focused. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [current, setCurrent] = useState<Toast | null>(null);
  const [paused, setPaused] = useState(false);
  const toast = useCallback((message: string, opts?: ToastOptions) => setCurrent({ id: ++seq, message, ...opts }), []);
  const dismiss = useCallback(() => { setCurrent(null); setPaused(false); }, []);

  useEffect(() => {
    if (!current || paused) return;
    const t = setTimeout(dismiss, current.action ? 8000 : 4000);
    return () => clearTimeout(t);
  }, [current, paused, dismiss]);

  const api = useMemo(() => ({ toast, current, dismiss, pause: setPaused }), [toast, current, dismiss]);
  return (
    <ToastContext.Provider value={api}>
      {children}
      <ToastView />
    </ToastContext.Provider>
  );
}

/**
 * The live region and the message. The provider renders one for the page; every Sheet renders another inside its dialog,
 * because an open modal dialog covers the page (and makes it inert): there the one in the dialog is the one seen and heard.
 */
export function ToastView() {
  const api = useContext(ToastContext);
  if (!api) return null;
  const { current: t, dismiss, pause } = api;
  return (
    <div className="toast-region" role="status" aria-live="polite">
      {t && (
        // biome-ignore lint/a11y/noStaticElementInteractions: hover and focus only pause the timer; the buttons inside are the controls
        <div className="toast" key={t.id} onMouseEnter={() => pause(true)} onMouseLeave={() => pause(false)} onFocus={() => pause(true)} onBlur={() => pause(false)}>
          <span className="toast-text">{t.message}</span>
          {t.action && <button type="button" className="toast-action" onClick={() => { dismiss(); t.action!.run(); }}>{t.action.label}</button>}
          <button type="button" className="toast-close" aria-label="Dispensar aviso" onClick={dismiss}>✕</button>
        </div>
      )}
    </div>
  );
}
