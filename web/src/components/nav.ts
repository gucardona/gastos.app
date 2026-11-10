import { useSearchParams } from "react-router";
import { isMonth } from "../lib/dates";
import type { IconName } from "./Icon";

/** The month in the address (`?m`, only when valid) for the navigation links, so switching screen keeps it. */
export function useMonthSearch() {
  const m = useSearchParams()[0].get("m");
  return m && isMonth(m) ? `?m=${m}` : "";
}

export interface NavItem {
  to: string;
  label: string;
  icon: IconName;
  end?: boolean;
}

/** The destinations of the app. Ajustes is reached through the avatar, and Recorrentes from Lançamentos. */
export function navItems(splitting: boolean, assistant: boolean): NavItem[] {
  return [
    { to: "/", label: "Visão", icon: "home", end: true },
    { to: "/lancamentos", label: "Lançamentos", icon: "list" },
    { to: "/caixa", label: "Caixa", icon: "tray" },
    ...(assistant ? [{ to: "/chat", label: "Chat", icon: "chat" as const }] : []),
    ...(splitting ? [{ to: "/divisao", label: "Divisão", icon: "split" as const }] : []),
  ];
}

/** Text for the pending counter: nothing when empty, capped at 99+. */
export function formatBadge(n: number): string {
  if (n <= 0) return "";
  return n > 99 ? "99+" : String(n);
}
