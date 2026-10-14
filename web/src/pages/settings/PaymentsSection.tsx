import { useState } from "react";
import type { PaymentMethod } from "../../api/types";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { PaymentSheet } from "../../components/PaymentSheet";
import { useApp } from "../../context";

export function PaymentsSection() {
  const { paymentMethods, canEdit } = useApp();
  const [showArchived, setShowArchived] = useState(false);
  const [open, setOpen] = useState<PaymentMethod | "new" | null>(null);
  const active = paymentMethods.filter((p) => !p.archived);
  const archived = paymentMethods.filter((p) => p.archived);

  const rows = (list: PaymentMethod[]) => (
    <ListGroup>
      {list.map((p) => (
        <ListRow key={p.id} icon={p.icon} label={p.name} value={p.archived ? "Arquivada" : undefined} onClick={canEdit ? () => setOpen(p) : undefined} chevron={canEdit} />
      ))}
    </ListGroup>
  );

  return (
    <div className="screen">
      {rows(active)}
      {canEdit && <button type="button" className="btn ghost full" onClick={() => setOpen("new")}>Nova forma de pagamento</button>}
      {archived.length > 0 && (
        <>
          <button type="button" className="chip" aria-expanded={showArchived} onClick={() => setShowArchived(!showArchived)}>
            {showArchived ? "Ocultar" : "Mostrar"} arquivadas ({archived.length})
          </button>
          {showArchived && rows(archived)}
        </>
      )}
      <PaymentSheet method={open} onClose={() => setOpen(null)} />
    </div>
  );
}
