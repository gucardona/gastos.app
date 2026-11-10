import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(join(__dirname, "..", "styles.css"), "utf8");

describe("small phones", () => {
  it("each tab can shrink, and its label ends in an ellipsis instead of spilling into the next one", () => {
    expect(css).toMatch(/\.tabbar-pill a\s*\{[^}]*min-width:\s*0[^}]*font-size:\s*clamp\(/);
    expect(css).toMatch(/\.tab-label\s*\{[^}]*overflow:\s*hidden[^}]*text-overflow:\s*ellipsis[^}]*white-space:\s*nowrap/);
  });

  it("the month title stays on one line and scales down", () => {
    expect(css).toMatch(/\.month-title\s*\{[^}]*font-size:\s*clamp\([^}]*white-space:\s*nowrap/);
  });

  it("sheets size by the dynamic viewport, with a vh fallback", () => {
    expect(css).toMatch(/dialog\.sheet\s*\{[^}]*max-height:\s*92vh;\s*max-height:\s*92dvh/);
    expect(css).toMatch(/dialog\.sheet\s*\{[^}]*height:\s*100vh;\s*height:\s*100dvh/);
  });

  it("the page clears the notch at the top", () => {
    expect(css).toMatch(/\.main\s*\{[^}]*padding:\s*calc\(20px \+ env\(safe-area-inset-top\)\)/);
  });
});

describe("tap targets", () => {
  it("the wallet button, chips and the avatar grow their hit area to 44px without growing their look", () => {
    expect(css).toMatch(/\.wallet-btn::after, \.chip::after\s*\{[^}]*position:\s*absolute;\s*inset:\s*-8px 0/);
    expect(css).toMatch(/\.avatar::after\s*\{[^}]*position:\s*absolute;\s*inset:\s*-4px/);
    expect(css).toMatch(/\.wallet-btn, \.chip, \.avatar\s*\{[^}]*position:\s*relative/);
  });
});

describe("the pie on the overview", () => {
  it("sits in a card, centered without text-align, side by side with its legend when the card is wide", () => {
    expect(css).toMatch(/\.chart-card\s*\{[^}]*container-type:\s*inline-size/);
    expect(css).toMatch(/\.chart-card \.chart-pie\s*\{[^}]*justify-self:\s*center[^}]*max-width:\s*200px/);
    expect(css).toMatch(/@container \(min-width: \d+px\)\s*\{\s*\.chart-card \.chart\s*\{[^}]*grid-template-columns/);
  });

  it("the card styling is scoped to the overview, so a chart in a chat bubble gets no card", () => {
    const chartRules = [...css.matchAll(/([^{}]+)\{[^{}]*\}/g)].map((m) => m[1].trim()).filter((s) => /\.chart/.test(s) && /container-type|background:\s*var\(--surface\)/.test(s));
    expect(chartRules.every((s) => s.startsWith(".chart-card"))).toBe(true);
  });
});
