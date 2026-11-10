import { useState } from "react";
import { ApiError } from "../api/client";
import { useRefresh } from "../api/queries";
import { useToast } from "../components/Toast";
import { useApp } from "../context";

/** What to tell the person when a call fails. */
export const errorMessage = (e: unknown) => (e instanceof ApiError ? (Object.values(e.fields)[0] ?? e.message) : "Sem conexão. Tente de novo.");

/** Runs a write against the API: tracks "busy", surfaces the error, refreshes the wallet's data and, given a `done` message, says it in a toast. */
export function useAction() {
  const { wallet } = useApp();
  const refresh = useRefresh();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function run(fn: () => Promise<unknown>, done?: string) {
    setBusy(true);
    setError("");
    try {
      await fn();
      refresh(wallet.id);
      if (done) toast(done);
      return true;
    } catch (e) {
      setError(errorMessage(e));
      return false;
    } finally {
      setBusy(false);
    }
  }
  return { run, busy, error };
}

/** A toast's "Desfazer" that recreates what was deleted with `recreate`; the sheet is gone by then, so a failure is told in a toast. */
export function useUndo() {
  const { wallet } = useApp();
  const refresh = useRefresh();
  const toast = useToast();
  return (recreate: () => Promise<unknown>) => ({
    label: "Desfazer",
    run: () => void recreate().then(() => refresh(wallet.id), (e) => toast(errorMessage(e))),
  });
}
