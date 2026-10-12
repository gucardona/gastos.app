import { useEffect, useMemo, useState } from "react";
import { Navigate, Outlet, useSearchParams } from "react-router";
import { isUnauthenticated } from "./api/client";
import { useCategories, useInbox, useMe, useMembers, usePaymentMethods, useWalletEvents } from "./api/queries";
import type { Transaction } from "./api/types";
import { AddSheet } from "./components/AddSheet";
import { Sidebar } from "./components/Sidebar";
import { TabBar } from "./components/TabBar";
import { Ctx, type AppCtx } from "./context";
import { monthOf, today } from "./lib/dates";

function readWalletId() {
  try {
    return Number(localStorage.getItem("walletId")) || 0;
  } catch {
    return 0;
  }
}

export function Shell() {
  const me = useMe();
  const [storedId, setStoredId] = useState(readWalletId);
  const [params, setParams] = useSearchParams();
  const [adding, setAdding] = useState<{ tx?: Transaction } | null>(null);

  const wallets = me.data?.wallets ?? [];
  const wallet = wallets.find((w) => w.id === storedId) ?? wallets[0];
  const walletId = wallet?.id ?? 0;

  const categories = useCategories(walletId);
  const paymentMethods = usePaymentMethods(walletId);
  const members = useMembers(walletId);
  useWalletEvents(walletId);
  const pending = useInbox(walletId).data?.length ?? 0;

  const month = params.get("m") ?? monthOf(today());
  const canEdit = wallet?.role !== "reader";

  // the N shortcut opens quick add, as in the spec
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement;
      if (e.key.toLowerCase() === "n" && canEdit && !e.metaKey && !e.ctrlKey && !/INPUT|TEXTAREA|SELECT/.test(el.tagName) && !document.querySelector("dialog[open]")) {
        e.preventDefault();
        setAdding({});
      }
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, [canEdit]);

  const ctx = useMemo<AppCtx | null>(() => {
    if (!me.data || !wallet || !categories.data || !paymentMethods.data || !members.data) return null;
    return {
      user: me.data.user,
      wallet,
      wallets,
      setWalletId: (id) => {
        setStoredId(id);
        try {
          localStorage.setItem("walletId", String(id));
        } catch {
          // no storage available: only lasts for this session
        }
      },
      month,
      setMonth: (m) => setParams((p) => { p.set("m", m); return p; }, { replace: true }),
      categories: categories.data,
      paymentMethods: paymentMethods.data,
      members: members.data,
      canEdit,
      openAdd: (tx) => setAdding({ tx }),
    };
  }, [me.data, wallet, wallets, categories.data, paymentMethods.data, members.data, month, canEdit, setParams]);

  if (isUnauthenticated(me.error)) return <Navigate to="/login" replace />;
  if (me.error) return <Failed onRetry={() => me.refetch()} />;
  if (me.data && wallets.length === 0) return <Failed message="Você não participa de nenhuma carteira." />;
  if (categories.error || paymentMethods.error || members.error) {
    return <Failed onRetry={() => { void categories.refetch(); void paymentMethods.refetch(); void members.refetch(); }} />;
  }
  if (!ctx) return <div className="main"><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></div>;

  return (
    <Ctx.Provider value={ctx}>
      <div className="app">
        <Sidebar pending={pending} onAdd={() => setAdding({})} />
        <main className="main"><Outlet /></main>
      </div>
      <TabBar pending={pending} onAdd={() => setAdding({})} />
      <AddSheet state={adding} onClose={() => setAdding(null)} />
    </Ctx.Provider>
  );
}

function Failed({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  return (
    <div className="main empty">
      <p>{message ?? "Não foi possível carregar seus dados. Confira a conexão e tente de novo."}</p>
      {onRetry && <button type="button" className="btn" onClick={onRetry}>Tentar de novo</button>}
    </div>
  );
}
