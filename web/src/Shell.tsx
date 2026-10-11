import { useEffect, useMemo, useState } from "react";
import { Navigate, NavLink, Outlet, useSearchParams } from "react-router";
import { isUnauthenticated } from "./api/client";
import { useCategories, useMe, useMembers, usePaymentMethods, useWalletEvents } from "./api/queries";
import type { Transaction } from "./api/types";
import { AddSheet } from "./components/AddSheet";
import { Ctx, type AppCtx } from "./context";
import { monthOf, today } from "./lib/dates";

const NAV = [
  { to: "/", label: "Visão", long: "Visão geral", icon: "◔", end: true },
  { to: "/lancamentos", label: "Lançamentos", long: "Lançamentos", icon: "≡" },
  { to: "/recorrentes", label: "Recorrentes", long: "Recorrentes", icon: "↻" },
  { to: "/divisao", label: "Divisão", long: "Divisão", icon: "⇄" },
];

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

  const month = params.get("m") ?? monthOf(today());
  const canEdit = wallet?.role !== "reader";

  // atalho N abre o lançamento rápido, como na spec
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
          // sem armazenamento: vale só nesta sessão
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [me.data, wallet, categories.data, paymentMethods.data, members.data, month, canEdit]);

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
        <aside className="side" aria-label="Navegação">
          <div className="brand">gastos</div>
          {NAV.map((n) => (
            <NavLink key={n.to} to={n.to} end={n.end}><span aria-hidden>{n.icon}</span>{n.long}</NavLink>
          ))}
          <div className="grow" />
          {canEdit && <button className="btn add" onClick={() => setAdding({})}>Novo lançamento</button>}
          <NavLink to="/ajustes"><span aria-hidden>⚙</span>Ajustes</NavLink>
        </aside>
        <main className="main"><Outlet /></main>
      </div>
      <nav className="bottom" aria-label="Navegação">
        <NavLink to="/" end><span aria-hidden>◔</span>Visão</NavLink>
        <NavLink to="/lancamentos"><span aria-hidden>≡</span>Lançamentos</NavLink>
        {canEdit ? <button className="fab" onClick={() => setAdding({})} aria-label="Novo lançamento">+</button> : <span />}
        <NavLink to="/recorrentes"><span aria-hidden>↻</span>Recorrentes</NavLink>
        <NavLink to="/divisao"><span aria-hidden>⇄</span>Divisão</NavLink>
      </nav>
      <AddSheet state={adding} onClose={() => setAdding(null)} />
    </Ctx.Provider>
  );
}

function Failed({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  return (
    <div className="main empty">
      <p>{message ?? "Não foi possível carregar seus dados. Confira a conexão e tente de novo."}</p>
      {onRetry && <button className="btn" onClick={onRetry}>Tentar de novo</button>}
    </div>
  );
}
