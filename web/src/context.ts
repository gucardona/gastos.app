import { createContext, useContext } from "react";
import type { Category, Member, PaymentMethod, Transaction, User, Wallet } from "./api/types";

export interface AppCtx {
  user: User;
  wallet: Wallet;
  wallets: Wallet[];
  setWalletId: (id: number) => void;
  month: string;
  setMonth: (m: string) => void;
  categories: Category[];
  paymentMethods: PaymentMethod[];
  members: Member[];
  canEdit: boolean;
  /** Abre o lançamento rápido; com `tx`, edita esse lançamento. */
  openAdd: (tx?: Transaction) => void;
}

export const Ctx = createContext<AppCtx | null>(null);

export function useApp() {
  const c = useContext(Ctx);
  if (!c) throw new Error("useApp fora do Shell");
  return c;
}

export const firstName = (full: string) => full.split(" ")[0];
