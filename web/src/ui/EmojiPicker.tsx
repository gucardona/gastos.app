import { useState } from "react";

export const EMOJI_TABS = [
  { label: "Casa", emojis: ["🏠", "⚡", "📺", "🛋️", "💡", "🧺", "🔧", "🛏️", "🌱", "🪴", "🧹", "🔑", "🚿", "🪑"] },
  { label: "Comida", emojis: ["🍽", "🛒", "🍜", "🍕", "🍔", "☕", "🍺", "🥗", "🍎", "🥖", "🍰", "🍳"] },
  { label: "Transporte", emojis: ["🚗", "✈️", "🚌", "🚇", "⛽", "🚲", "🛵", "🚕", "🚆", "🛳️", "🅿️", "🛠️"] },
  { label: "Lazer", emojis: ["🎮", "📚", "🎬", "🎵", "⚽", "🎟️", "🏖️", "🎨", "🎲", "🏋️", "🎤", "🧩"] },
  { label: "Saúde", emojis: ["💊", "🩺", "🏥", "🦷", "🧴", "💪", "🧘", "👓", "🩹", "🧬", "❤️", "🧠"] },
  { label: "Dinheiro", emojis: ["💼", "📈", "💳", "🏦", "💵", "📄", "💰", "🧾", "🪙", "🎁", "📊", "🏷️", "🧮"] },
  { label: "Outros", emojis: ["👕", "✨", "◈", "🧑‍💻", "↩️", "↔", "🐶", "🐱", "👶", "🎓", "💇", "📱", "💻", "🌎", "🔔", "⭐", "🎄"] },
] as const;

/** Icons for a category or payment method: pick a subject, then tap an emoji. */
export function EmojiPicker({ value, onPick }: { value: string; onPick: (emoji: string) => void }) {
  const known = EMOJI_TABS.findIndex((t) => (t.emojis as readonly string[]).includes(value));
  const custom = value !== "" && known < 0; // typed freely in the old form: keep it selectable
  const [tab, setTab] = useState(custom ? EMOJI_TABS.length - 1 : Math.max(known, 0));
  const emojis: readonly string[] = custom && tab === EMOJI_TABS.length - 1 ? [value, ...EMOJI_TABS[tab].emojis] : EMOJI_TABS[tab].emojis;

  return (
    <div className="emoji-picker">
      <div className="emoji-tabs" role="group" aria-label="Assuntos">
        {EMOJI_TABS.map((t, i) => (
          <button type="button" key={t.label} aria-pressed={i === tab} onClick={() => setTab(i)}>{t.label}</button>
        ))}
      </div>
      <div className="emoji-grid" role="group" aria-label="Ícones">
        {emojis.map((e) => (
          <button type="button" key={e} aria-pressed={e === value} onClick={() => onPick(e)}>{e}</button>
        ))}
      </div>
    </div>
  );
}
