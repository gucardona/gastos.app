import { formatBRL } from "../lib/money";

/** "Entrou" and "Saiu" of the month, each with what is still to come under it when there is any. */
export function Flows({ income, expense, toReceive, toPay }: { income: number; expense: number; toReceive: number; toPay: number }) {
  return (
    <div className="flows">
      <span className="in">Entrou <b className="num">{formatBRL(income)}</b>{toReceive > 0 && <small className="ahead num">+ {formatBRL(toReceive)} a receber</small>}</span>
      <span className="out">Saiu <b className="num">{formatBRL(expense)}</b>{toPay > 0 && <small className="ahead num">+ {formatBRL(toPay)} a pagar</small>}</span>
    </div>
  );
}
