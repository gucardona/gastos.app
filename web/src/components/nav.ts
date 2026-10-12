import type { IconName } from "./Icon";

export interface NavItem {
  to: string;
  label: string;
  icon: IconName;
  end?: boolean;
}

/** The destinations of the app. Ajustes is reached through the avatar, and Recorrentes from Lançamentos. */
export function navItems(splitting: boolean): NavItem[] {
  return [
    { to: "/", label: "Visão", icon: "home", end: true },
    { to: "/lancamentos", label: "Lançamentos", icon: "list" },
    { to: "/caixa", label: "Caixa", icon: "tray" },
    ...(splitting ? [{ to: "/divisao", label: "Divisão", icon: "split" as const }] : []),
  ];
}

/** Text for the pending counter: nothing when empty, capped at 99+. */
export function formatBadge(n: number): string {
  if (n <= 0) return "";
  return n > 99 ? "99+" : String(n);
}
