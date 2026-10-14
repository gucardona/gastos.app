import { useState } from "react";
import { Link } from "react-router";
import { api } from "../api/client";
import { useSettlements, useSummary } from "../api/queries";
import type { Settlement } from "../api/types";
import { ListGroup, ListRow } from "../components/ListGroup";
import { PageBar } from "../components/PageBar";
import { SettlementSheet } from "../components/SettlementSheet";
import { firstName, useApp } from "../context";
import { useAction } from "../hooks/useAction";
import { dayLabel, today } from "../lib/dates";
import { formatBRL } from "../lib/money";

export function Division() {
  const { wallet, month, members, user, canEdit } = useApp();
  const summary = useSummary(wallet.id, month);
  const history = useSettlements(wallet.id);
  const register = useAction();
  const [sheet, setSheet] = useState<Settlement | "new" | null>(null);

  const who = (id: number) => (id === user.id ? "Você" : firstName(members.find((m) => m.userId === id)?.name ?? "Alguém"));

  if (!wallet.splittingEnabled) {
    return (
      <>
        <PageBar />
        <Title />
        <div className="empty">
          <p>A divisão de gastos está desligada em {wallet.name}. Ligue para ver quem pagou o quê e acertar as contas entre vocês.</p>
          {wallet.role === "owner" && <Link className="btn" to="/ajustes/carteira">Ligar a divisão</Link>}
        </div>
      </>
    );
  }
  if (summary.error || history.error) {
    return <><PageBar /><Title /><div className="empty"><p>Não foi possível carregar a divisão.</p><button type="button" className="btn" onClick={() => { void summary.refetch(); void history.refetch(); }}>Tentar de novo</button></div></>;
  }
  if (!summary.data || !history.data) return <><PageBar /><Title /><div className="skeleton" /><div className="skeleton" style={{ width: "40%" }} /></>;

  const balances = summary.data.memberBalances ?? [];
  const suggestions = summary.data.suggestedSettlements ?? [];

  return (
    <>
      <PageBar />
      <Title />

      <section className="sec">
        <h2>Para acertar</h2>
        {suggestions.length === 0 ? (
          <p className="big">Vocês estão quites.</p>
        ) : (
          suggestions.map((t) => (
            <div className="settle" key={`${t.fromUserId}-${t.toUserId}`}>
              <p className="big">{who(t.fromUserId)} deve {formatBRL(t.amountCents)} {t.toUserId === user.id ? "a você" : `para ${who(t.toUserId)}`}.</p>
              {canEdit && (
                <button type="button" className="btn full" disabled={register.busy}
                  onClick={() => void register.run(() => api.post(`/wallets/${wallet.id}/settlements`, { fromUserId: t.fromUserId, toUserId: t.toUserId, amountCents: t.amountCents, date: today() }))}>
                  Registrar acerto
                </button>
              )}
            </div>
          ))
        )}
        {register.error && <p className="error" role="alert">{register.error}</p>}
        {canEdit && <button type="button" className="btn ghost full" onClick={() => setSheet("new")}>Registrar outro acerto</button>}
      </section>

      <section className="sec">
        <h2>Saldo entre vocês</h2>
        <p className="sub">Soma de todos os gastos divididos e acertos já registrados.</p>
        <ListGroup>
          {balances.map((b) => (
            <ListRow
              key={b.userId}
              label={<><span className="desc">{who(b.userId)}</span><span className="meta">{b.netCents > 0 ? "tem a receber" : b.netCents < 0 ? "deve" : "está quite"}</span></>}
              value={<span className={`num ${b.netCents > 0 ? "plus" : ""}`}>{b.netCents === 0 ? formatBRL(0) : `${b.netCents > 0 ? "+" : "−"}${formatBRL(Math.abs(b.netCents))}`}</span>}
            />
          ))}
        </ListGroup>
      </section>

      <section className="sec">
        <h2>Histórico de acertos</h2>
        {history.data.length === 0 ? (
          <p className="sub">Nenhum acerto registrado ainda.</p>
        ) : (
          <ListGroup>
            {history.data.map((s) => (
              <ListRow
                key={s.id}
                label={<><span className="desc">{who(s.fromUserId)} pagou {s.toUserId === user.id ? "a você" : who(s.toUserId)}</span><span className="meta">{dayLabel(s.date)}{s.note && `. ${s.note}`}{s.recurrenceId && ". Recorrente"}</span></>}
                value={<span className="num">{formatBRL(s.amountCents)}</span>}
                onClick={canEdit ? () => setSheet(s) : undefined}
                chevron={canEdit}
              />
            ))}
          </ListGroup>
        )}
      </section>
      <SettlementSheet settlement={sheet} onClose={() => setSheet(null)} />
    </>
  );
}

const Title = () => <h1 className="page-title">Divisão</h1>;
