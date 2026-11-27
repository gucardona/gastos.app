import { useState } from "react";
import type { PaymentMethod } from "../../api/types";
import { ListGroup, ListRow } from "../../ui/ListGroup";
import { PaymentSheet } from "../../features/payments/PaymentSheet";
import { useApp } from "../../app/context";

/** "fecha 20 · vence 27 · final 1234" for a card. */
const cardSummary = (p: PaymentMethod) =>
  p.card ? [`fecha ${p.card.closingDay}`, `vence ${p.card.dueDay}`, p.card.last4 && `final ${p.card.last4}`].filter(Boolean).join(" · ") : undefined;

/** Formas de pagamento, or with `cards` the Cartões de crédito screen. A card also lists among the methods, but is edited only as a card. */
export function PaymentsSection({ cards = false }: { cards?: boolean }) {
  const { paymentMethods, canEdit } = useApp();
  const [showArchived, setShowArchived] = useState(false);
  const [open, setOpen] = useState<PaymentMethod | "new" | null>(null);
  const list = cards ? paymentMethods.filter((p) => p.card) : paymentMethods;
  const active = list.filter((p) => !p.archived);
  const archived = list.filter((p) => p.archived);

  const rows = (items: PaymentMethod[]) => (
    <ListGroup>
      {items.map((p) => p.card && !cards
        ? <ListRow key={p.id} icon={p.icon} label={p.name} value={p.archived ? "Arquivado" : "Em Cartões de crédito"} to="/ajustes/cartoes" chevron />
        : <ListRow key={p.id} icon={p.icon} label={p.name} value={p.archived ? (cards ? "Arquivado" : "Arquivada") : cardSummary(p)} onClick={canEdit ? () => setOpen(p) : undefined} chevron={canEdit} />)}
    </ListGroup>
  );

  return (
    <div className="screen">
      {active.length === 0 && cards ? <p className="sub">Nenhum cartão ainda. Cada cartão vira uma forma de pagamento, e as compras nele contam no mês em que a fatura vence.</p> : rows(active)}
      {canEdit && <button type="button" className="btn ghost full" onClick={() => setOpen("new")}>{cards ? "Novo cartão" : "Nova forma de pagamento"}</button>}
      {archived.length > 0 && (
        <>
          <button type="button" className="chip" aria-expanded={showArchived} onClick={() => setShowArchived(!showArchived)}>
            {showArchived ? "Ocultar" : "Mostrar"} {cards ? "arquivados" : "arquivadas"} ({archived.length})
          </button>
          {showArchived && rows(archived)}
        </>
      )}
      <PaymentSheet method={open} card={cards} onClose={() => setOpen(null)} />
    </div>
  );
}
