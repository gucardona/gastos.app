// Category colors are data chosen by the person, not interface tokens: a fixed palette that reads well on light and dark.
export const SWATCH_COLORS = ["#0E6577", "#3B6FD8", "#7A5AD6", "#D4568B", "#D9822B", "#2F9D68", "#8B949A", "#C2412D"];

/** A row of color swatches; a color from the old free picker stays as an extra, marked swatch. */
export function ColorSwatches({ value, onPick }: { value: string; onPick: (hex: string) => void }) {
  const inPalette = SWATCH_COLORS.some((c) => c.toLowerCase() === value.toLowerCase());
  const colors = inPalette || !value ? SWATCH_COLORS : [...SWATCH_COLORS, value];
  return (
    <div className="swatches">
      {colors.map((c) => (
        <button type="button" key={c} className="swatch-btn" aria-label={`Cor ${c}`} aria-pressed={c.toLowerCase() === value.toLowerCase()} onClick={() => onPick(c)}>
          <span style={{ background: c }} />
        </button>
      ))}
    </div>
  );
}
