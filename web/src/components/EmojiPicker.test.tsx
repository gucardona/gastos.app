import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { EMOJI_TABS, EmojiPicker } from "./EmojiPicker";

const tabs = () => screen.getByRole("group", { name: "Assuntos" });
const grid = () => screen.getByRole("group", { name: "Ícones" });
const pressed = (el: HTMLElement) => within(el).getAllByRole("button").filter((b) => b.getAttribute("aria-pressed") === "true").map((b) => b.textContent);

// what a new wallet is seeded with (internal/api/wallets.go): all of it must be pickable, or editing a default category would lose its icon
const SEEDED = ["🍽", "🛒", "🚗", "💊", "🏠", "⚡", "🍜", "🎮", "📺", "👕", "✨", "✈️", "📚", "📈", "◈", "💼", "🧑‍💻", "↩️", "💳", "🏦", "💵", "📄", "↔"];

describe("EmojiPicker data", () => {
  it("has the seven subjects, each with at least 12 emojis, none repeated", () => {
    expect(EMOJI_TABS.map((t) => t.label)).toEqual(["Casa", "Comida", "Transporte", "Lazer", "Saúde", "Dinheiro", "Outros"]);
    const all = EMOJI_TABS.flatMap((t) => t.emojis);
    for (const t of EMOJI_TABS) expect(t.emojis.length, t.label).toBeGreaterThanOrEqual(12);
    expect(new Set(all).size).toBe(all.length);
  });

  it("includes every icon a new wallet is seeded with", () => {
    const all = new Set<string>(EMOJI_TABS.flatMap((t) => t.emojis));
    for (const icon of SEEDED) expect(all.has(icon), icon).toBe(true);
  });
});

describe("EmojiPicker", () => {
  it("opens on the subject that holds the current emoji, with only it marked", () => {
    render(<EmojiPicker value="🛒" onPick={vi.fn()} />);
    expect(pressed(tabs())).toEqual(["Comida"]);
    expect(pressed(grid())).toEqual(["🛒"]);
  });

  it("switching subject shows its emojis", async () => {
    render(<EmojiPicker value="🛒" onPick={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: "Transporte" }));
    expect(screen.getByRole("button", { name: "🚗" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "🛒" })).toBeNull();
    expect(pressed(grid())).toEqual([]);
  });

  it("reports the emoji tapped", async () => {
    const onPick = vi.fn();
    render(<EmojiPicker value="🛒" onPick={onPick} />);
    await userEvent.click(screen.getByRole("button", { name: "🍜" }));
    expect(onPick).toHaveBeenCalledWith("🍜");
  });

  it("an emoji outside every subject (typed freely in the old form) shows first, marked, and is not lost", async () => {
    const onPick = vi.fn();
    render(<EmojiPicker value="🦄" onPick={onPick} />);
    expect(within(grid()).getAllByRole("button")[0].textContent).toBe("🦄");
    expect(pressed(grid())).toEqual(["🦄"]);
    expect(pressed(tabs())).toEqual(["Outros"]);
    await userEvent.click(within(grid()).getAllByRole("button")[0]);
    expect(onPick).toHaveBeenCalledWith("🦄");
  });

  it("with no emoji chosen yet it opens on Casa with nothing marked", () => {
    render(<EmojiPicker value="" onPick={vi.fn()} />);
    expect(pressed(tabs())).toEqual(["Casa"]);
    expect(pressed(grid())).toEqual([]);
  });
});
