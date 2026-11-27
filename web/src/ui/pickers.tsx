import { type ReactNode, useEffect, useRef } from "react";
import type { Category } from "../api/types";
import { today, yesterday } from "../lib/dates";
import { ListGroup, ListRow } from "./ListGroup";

/** A choice that takes over the sheet: a back button and a title above the options. */
export function PickerView({ title, onBack, children }: { title: string; onBack: () => void; children: ReactNode }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { heading.current?.focus(); }, []); // the previous control is gone: land on the title, so the change is announced
  return (
    <div className="picker">
      <div className="picker-head">
        <button type="button" className="picker-back" onClick={onBack}>
          <span aria-hidden="true">‹</span> Voltar
        </button>
        <h2 ref={heading} tabIndex={-1}>{title}</h2>
      </div>
      {children}
    </div>
  );
}

/** Categories as touch tiles; scrolls when there are many. */
export function CategoryGrid({ categories, value, onPick }: { categories: Category[]; value: number | null; onPick: (id: number) => void }) {
  return (
    <div className="category-grid">
      {categories.map((c) => (
        <button type="button" className="cat-tile" key={c.id} aria-pressed={c.id === value} onClick={() => onPick(c.id)}>
          <span className="cat-emoji" aria-hidden="true">{c.icon}</span>
          {c.name}
        </button>
      ))}
    </div>
  );
}

/** A short list where exactly one option is chosen. */
export function OptionList<T extends string | number>({ options, value, onPick }: { options: { value: T; label: string }[]; value: T | null; onPick: (v: T) => void }) {
  return (
    <ListGroup>
      {options.map((o) => (
        <button type="button" className="list-row" key={o.value} aria-pressed={o.value === value} onClick={() => onPick(o.value)}>
          <span className="lr-label">{o.label}</span>
          {o.value === value && <span className="lr-check" aria-hidden="true">✓</span>}
        </button>
      ))}
    </ListGroup>
  );
}

/** Hoje, Ontem, or any day through the native date field. */
export function DatePicker({ value, onPick }: { value: string; onPick: (iso: string) => void }) {
  return (
    <>
      <OptionList
        options={[{ value: today(), label: "Hoje" }, { value: yesterday(), label: "Ontem" }]}
        value={value}
        onPick={onPick}
      />
      <ListGroup className="date-other">
        <ListRow label="Outro dia" trailing={<input type="date" className="date-input" aria-label="Data do lançamento" value={value} onChange={(e) => e.target.value && onPick(e.target.value)} />} />
      </ListGroup>
    </>
  );
}
