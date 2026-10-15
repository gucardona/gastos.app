export type IconName = "home" | "list" | "tray" | "chat" | "split" | "plus";

const PATHS: Record<IconName, React.ReactNode> = {
  home: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 3.5v8.5h8.5" />
    </>
  ),
  list: (
    <>
      <path d="M8 7h12M8 12h12M8 17h12" />
      <circle cx="4" cy="7" r=".6" />
      <circle cx="4" cy="12" r=".6" />
      <circle cx="4" cy="17" r=".6" />
    </>
  ),
  tray: (
    <>
      <path d="M3.5 13.5 6 5.5h12l2.5 8" />
      <path d="M3.5 13.5v5h17v-5h-5.5a3 3 0 0 1-6 0z" />
    </>
  ),
  chat: <path d="M4.5 6.5a2 2 0 0 1 2-2h11a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H10l-4 3.5v-3.5a2 2 0 0 1-1.5-2z" />,
  split: <path d="M4 8h13l-3.5-3.5M20 16H7l3.5 3.5" />,
  plus: <path d="M12 5v14M5 12h14" />,
};

/** Line icons, drawn at 24x24 and sized by CSS. Decorative: the label next to them names the control. */
export function Icon({ name }: { name: IconName }) {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {PATHS[name]}
    </svg>
  );
}
