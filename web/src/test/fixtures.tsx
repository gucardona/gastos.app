import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import { vi } from "vitest";
import type { Category, Member, PaymentMethod, Transaction, Wallet } from "../api/types";
import { Ctx, type AppCtx } from "../context";
import { today } from "../lib/dates";

export const ana = { id: 1, name: "Ana Souza", email: "ana@example.com" };

export const wallet: Wallet = { id: 1, name: "Casa", role: "owner", splittingEnabled: false, ownerName: "Ana Souza" };

const cat = (id: number, kind: Category["kind"], name: string, icon: string): Category => ({
  id, kind, name, icon, color: "#112233", essentiality: kind === "expense" ? "essential" : null, sortOrder: id, archived: false,
});
export const categories = [cat(1, "expense", "Mercado", "🛒"), cat(2, "expense", "Restaurante", "🍜"), cat(3, "income", "Salário", "💼")];

export const paymentMethods: PaymentMethod[] = [
  { id: 1, name: "Pix", icon: "⚡", sortOrder: 0, archived: false },
  { id: 2, name: "Crédito", icon: "💳", sortOrder: 1, archived: false },
];

export const members: Member[] = [
  { userId: 1, name: "Ana Souza", email: "ana@example.com", role: "owner", splitsByDefault: true },
  { userId: 2, name: "Bia Lima", email: "bia@example.com", role: "editor", splitsByDefault: true },
];

export function tx(over: Partial<Transaction> = {}): Transaction {
  return {
    id: 10, kind: "expense", amountCents: 4890, description: "Mercado do bairro", categoryId: 1, paymentMethodId: 1,
    date: today(), paidByUserId: 1, createdBy: 1, recurrenceId: null, splits: [], ...over,
  };
}

export function appCtx(over: Partial<AppCtx> = {}): AppCtx {
  return {
    user: ana, wallet, wallets: [wallet], setWalletId: vi.fn(), month: today().slice(0, 7), setMonth: vi.fn(),
    categories, paymentMethods, members, canEdit: true, openAdd: vi.fn(), ...over,
  };
}

/** Renderiza dentro do contexto do app, do React Query e de um roteador em memória. */
export function renderApp(ui: ReactElement, ctx: AppCtx = appCtx()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <Ctx.Provider value={ctx}>{ui}</Ctx.Provider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

type Reply = { status?: number; body?: unknown };

/** Troca o fetch global; devolve o mock para conferir as chamadas. */
export function mockFetch(reply: Reply | ((url: string, init: RequestInit) => Reply) = {}) {
  const fn = vi.fn(async (url: string, init: RequestInit = {}) => {
    const r = typeof reply === "function" ? reply(url, init) : reply;
    const status = r.status ?? 200;
    return new Response(status === 204 ? null : JSON.stringify(r.body ?? {}), { status, headers: { "Content-Type": "application/json" } });
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

/** Corpo JSON da n-ésima chamada ao fetch. */
export const bodyOf = (fn: ReturnType<typeof mockFetch>, call = 0) => JSON.parse(fn.mock.calls[call][1]!.body as string);
