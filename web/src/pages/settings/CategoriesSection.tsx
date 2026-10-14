import { useState } from "react";
import type { Category, Kind } from "../../api/types";
import { CategorySheet } from "../../components/CategorySheet";
import { ListGroup, ListRow } from "../../components/ListGroup";
import { Segmented } from "../../components/Segmented";
import { useApp } from "../../context";

const ESSENTIALITY = { essential: "Essencial", nonessential: "Supérfluo", investment: "Investimento" } as const;

export function CategoriesSection() {
  const { categories, canEdit } = useApp();
  const [kind, setKind] = useState<Kind>("expense");
  const [showArchived, setShowArchived] = useState(false);
  const [open, setOpen] = useState<Category | "new" | null>(null);
  const mine = categories.filter((c) => c.kind === kind);
  const active = mine.filter((c) => !c.archived);
  const archived = mine.filter((c) => c.archived);

  const rows = (list: Category[]) => (
    <ListGroup>
      {list.map((c) => (
        <ListRow
          key={c.id}
          icon={c.icon}
          label={c.name}
          value={c.archived ? "Arquivada" : c.essentiality ? ESSENTIALITY[c.essentiality] : "Entrada"}
          trailing={<span className="color-dot" style={{ background: c.color }} aria-hidden="true" />}
          onClick={canEdit ? () => setOpen(c) : undefined}
          chevron={canEdit}
        />
      ))}
    </ListGroup>
  );

  return (
    <div className="screen">
      <Segmented<Kind> label="Tipo de categoria" value={kind} onChange={setKind} options={[{ value: "expense", label: "Gastos" }, { value: "income", label: "Entradas" }]} />
      {rows(active)}
      {canEdit && <button type="button" className="btn ghost full" onClick={() => setOpen("new")}>Nova categoria</button>}
      {archived.length > 0 && (
        <>
          <button type="button" className="chip" aria-expanded={showArchived} onClick={() => setShowArchived(!showArchived)}>
            {showArchived ? "Ocultar" : "Mostrar"} arquivadas ({archived.length})
          </button>
          {showArchived && rows(archived)}
        </>
      )}
      <CategorySheet category={open} kind={kind} onClose={() => setOpen(null)} />
    </div>
  );
}
