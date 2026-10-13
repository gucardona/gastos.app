import { type ReactNode, useEffect, useRef } from "react";

/** A modal sheet on a native <dialog> (focus trap and Esc for free): rises from the bottom on phones, a side panel on desktop. */
export function Sheet({ open, onClose, label, children }: { open: boolean; onClose: () => void; label: string; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: a click on the backdrop is a pointer shortcut; keyboard users close with Esc (native dialog)
    <dialog ref={ref} className="sheet" aria-label={label} onClose={onClose} onClick={(e) => e.target === ref.current && onClose()}>
      {open && (
        <>
          <div className="grab" />
          {children}
        </>
      )}
    </dialog>
  );
}
