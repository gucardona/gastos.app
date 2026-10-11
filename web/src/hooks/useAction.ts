import { useState } from "react";
import { ApiError } from "../api/client";
import { useRefresh } from "../api/queries";
import { useApp } from "../context";

const message = (e: ApiError) => Object.values(e.fields)[0] ?? e.message;

/** Executa uma escrita na API: controla "ocupado", mostra o erro e renova os dados da carteira. */
export function useAction() {
  const { wallet } = useApp();
  const refresh = useRefresh();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await fn();
      refresh(wallet.id);
      return true;
    } catch (e) {
      setError(e instanceof ApiError ? message(e) : "Sem conexão. Tente de novo.");
      return false;
    } finally {
      setBusy(false);
    }
  }
  return { run, busy, error };
}
