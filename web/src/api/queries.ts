import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { monthRange } from "../lib/dates";
import { api } from "./client";
import type { Budget, Category, Me, Member, PaymentMethod, Recurrence, Summary, Transaction } from "./types";

// Toda chave de dados de uma carteira começa com ["wallet", id], então uma invalidação só renova tudo.
const w = (id: number, ...rest: unknown[]) => ["wallet", id, ...rest] as const;

export const useMe = () => useQuery({ queryKey: ["me"], queryFn: () => api.get<Me>("/me"), staleTime: 60_000 });

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

export const useSummary = (id: number, month: string) =>
  useQuery({ queryKey: w(id, "summary", month), queryFn: () => api.get<Summary>(`/wallets/${id}/summary?month=${month}`) });

export const useTransactions = (id: number, month: string) => {
  const { from, to } = monthRange(month);
  return useQuery({
    queryKey: w(id, "transactions", month),
    queryFn: () => api.get<Transaction[]>(`/wallets/${id}/transactions?from=${from}&to=${to}`),
  });
};

/** Renova os dados da carteira ativa (e `me`, que traz nome e papel das carteiras). */
export function useRefresh() {
  const qc = useQueryClient();
  return (walletId: number) => {
    void qc.invalidateQueries({ queryKey: ["wallet", walletId] });
    void qc.invalidateQueries({ queryKey: ["me"] });
  };
}

/** Atualiza a tela quando outro membro altera a carteira. O navegador reconecta o SSE sozinho. */
export function useWalletEvents(walletId: number) {
  const refresh = useRefresh();
  useEffect(() => {
    if (!walletId) return;
    const es = new EventSource(`/api/wallets/${walletId}/events`, { withCredentials: true });
    es.onmessage = () => refresh(walletId);
    return () => es.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [walletId]);
}
