// Mirrors the API v2 contract (docs/superpowers/specs/2026-10-09-backend-foundation-design.md). Money is always in cents.

export type Kind = "expense" | "income";
export type Role = "owner" | "editor" | "reader";

export interface User {
  id: number;
  name: string;
  email: string;
}

export interface Wallet {
  id: number;
  name: string;
  role: Role;
  splittingEnabled: boolean;
  ownerName: string;
}

export interface Me {
  user: User;
  wallets: Wallet[];
}

export interface Member {
  userId: number;
  name: string;
  email: string;
  role: Role;
  splitsByDefault: boolean;
}

export interface Category {
  id: number;
  kind: Kind;
  name: string;
  icon: string;
  color: string;
  essentiality: "essential" | "nonessential" | "investment" | null;
  sortOrder: number;
  archived: boolean;
}

export interface PaymentMethod {
  id: number;
  name: string;
  icon: string;
  sortOrder: number;
  archived: boolean;
}

export interface Split {
  userId: number;
  shareBp: number;
  amountCents: number;
}

export interface Transaction {
  id: number;
  kind: Kind;
  amountCents: number;
  description: string;
  categoryId: number;
  paymentMethodId: number | null;
  date: string;
  paidByUserId: number;
  createdBy: number;
  recurrenceId: number | null;
  splits: Split[];
}

export interface TransactionInput {
  kind: Kind;
  amountCents: number;
  description: string;
  categoryId: number;
  paymentMethodId: number | null;
  date: string;
  paidByUserId: number;
  splits?: { userId: number; shareBp: number }[];
}

export interface Transfer {
  fromUserId: number;
  toUserId: number;
  amountCents: number;
}

export interface Summary {
  month: string;
  incomeCents: number;
  expenseCents: number;
  balanceCents: number;
  dailyAverageCents: number;
  byCategory: { categoryId: number; kind: Kind; cents: number; budgetCents: number | null }[];
  byEssentiality: Record<string, number>;
  previous: { incomeCents: number; expenseCents: number };
  memberBalances?: { userId: number; netCents: number }[];
  suggestedSettlements?: Transfer[];
}

export interface Recurrence {
  id: number;
  kind: Kind | "settlement";
  amountCents: number;
  description: string;
  categoryId: number | null;
  paymentMethodId: number | null;
  paidByUserId: number;
  toUserId: number | null;
  dayOfMonth: number;
  startDate: string;
  endDate: string | null;
  enabled: boolean;
  nextDate: string | null;
  splits: Split[];
}

export interface Budget {
  categoryId: number;
  limitCents: number;
}

export interface Settlement {
  id: number;
  fromUserId: number;
  toUserId: number;
  amountCents: number;
  date: string;
  note: string;
  recurrenceId: number | null;
  createdBy: number;
}
