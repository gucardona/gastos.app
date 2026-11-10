import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect } from "react";
import { addDays, monthRange, today } from "../lib/dates";
import { api } from "./client";
import type { AISettings, BankAccount, Budget, Category, ChatMessage, InboxItem, Me, Member, PaymentMethod, Recurrence, Rule, Settlement, Summary, Transaction } from "./types";

// Every data key of a wallet starts with ["wallet", id], so a single invalidation refreshes everything.
const w = (id: number, ...rest: unknown[]) => ["wallet", id, ...rest] as const;

export const useMe = () => useQuery({ queryKey: ["me"], queryFn: () => api.get<Me>("/me"), staleTime: 60_000 });
export const useAISettings = () => useQuery({ queryKey: ["ai"], queryFn: () => api.get<AISettings>("/me/ai") });

export const useCategories = (id: number) =>
  useQuery({ queryKey: w(id, "categories"), queryFn: () => api.get<Category[]>(`/wallets/${id}/categories`), staleTime: 60_000, enabled: id > 0 });

export const usePaymentMethods = (id: number) =>
  useQuery({ queryKey: w(id, "payment-methods"), queryFn: () => api.get<PaymentMethod[]>(`/wallets/${id}/payment-methods`), staleTime: 60_000, enabled: id > 0 });

export const useMembers = (id: number) =>
  useQuery({ queryKey: w(id, "members"), queryFn: () => api.get<Member[]>(`/wallets/${id}/members`), staleTime: 60_000, enabled: id > 0 });

export const useRecurrences = (id: number) =>
  useQuery({ queryKey: w(id, "recurrences"), queryFn: () => api.get<Recurrence[]>(`/wallets/${id}/recurrences`) });

export const useBudgets = (id: number) =>
  useQuery({ queryKey: w(id, "budgets"), queryFn: () => api.get<Budget[]>(`/wallets/${id}/budgets`) });

/** Settlements from the last ~10 months (the API caps the range at 366 days) and the next 2. */
export const useSettlements = (id: number) => {
  const from = addDays(today(), -300);
  const to = addDays(today(), 60);
  return useQuery({ queryKey: w(id, "settlements"), queryFn: () => api.get<Settlement[]>(`/wallets/${id}/settlements?from=${from}&to=${to}`) });
};

export const useSummary = (id: number, month: string) =>
  useQuery({ queryKey: w(id, "summary", month), queryFn: () => api.get<Summary>(`/wallets/${id}/summary?month=${month}`) });

export const useTransactions = (id: number, month: string) => {
  const { from, to } = monthRange(month);
  return useQuery({
    queryKey: w(id, "transactions", month),
    queryFn: () => api.get<Transaction[]>(`/wallets/${id}/transactions?from=${from}&to=${to}`),
  });
};

/** Refreshes the active wallet's data (and `me`, which carries the wallets' name and role). */
export function useRefresh() {
  const qc = useQueryClient();
  return useCallback(
    (walletId: number) => {
      void qc.invalidateQueries({ queryKey: ["wallet", walletId] });
      void qc.invalidateQueries({ queryKey: ["me"] });
    },
    [qc],
  );
}

/**
 * Keeps every tab in sync: one stream per signed-in user tells it when anything they can see changed
 * (in this tab, another tab, or by another member). Also catches up after a reconnect and when the page
 * comes back after a while hidden. The browser reconnects the SSE by itself.
 */
export function useEvents(enabled: boolean) {
  const qc = useQueryClient();
  useEffect(() => {
    if (!enabled) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const refresh = () => {
      clearTimeout(timer); // trailing debounce: a burst of notices becomes one round of refetches
      timer = setTimeout(async () => {
        // the wallet list first, so a wallet deleted elsewhere is not refetched (it would only 404) while the Shell moves away from it
        await qc.invalidateQueries({ queryKey: ["me"] });
        const ids = new Set(qc.getQueryData<Me>(["me"])?.wallets.map((w) => w.id));
        const alive = (q: { queryKey: readonly unknown[] }) => ids.has(q.queryKey[1] as number);
        void qc.invalidateQueries({ queryKey: ["wallet"], predicate: alive });
        void qc.invalidateQueries({ queryKey: ["wallet"], predicate: (q) => !alive(q), refetchType: "none" });
      }, 150);
    };

    let opened = false;
    const es = new EventSource("/api/events", { withCredentials: true });
    es.onmessage = refresh;
    es.onopen = () => {
      if (opened) refresh(); // a reconnect: whatever changed while disconnected was never announced
      opened = true;
    };

    let hiddenAt = 0;
    const onVisibility = () => {
      if (document.hidden) hiddenAt = Date.now();
      else if (hiddenAt && Date.now() - hiddenAt > 30_000) refresh(); // a sleeping device may have missed notices
    };
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      es.close();
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [enabled, qc]);
}

export const useBankAccounts = (id: number) =>
  useQuery({ queryKey: w(id, "bank-accounts"), queryFn: () => api.get<BankAccount[]>(`/wallets/${id}/bank-accounts`) });

/** Statement lines waiting for a decision. */
export const useInbox = (id: number) =>
  useQuery({ queryKey: w(id, "inbox"), queryFn: () => api.get<InboxItem[]>(`/wallets/${id}/inbox`), enabled: id > 0 });

export const useRules = (id: number) =>
  useQuery({ queryKey: w(id, "rules"), queryFn: () => api.get<Rule[]>(`/wallets/${id}/rules`) });

/** The user's conversation with the assistant in a wallet. */
export const useChat = (walletId: number) =>
  useQuery({
    queryKey: ["chat", walletId],
    queryFn: () => api.get<ChatMessage[]>(`/chat?wallet=${walletId}`),
    // keeps a stopped or failed exchange, which the server does not store, on screen while the user is around
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  });
