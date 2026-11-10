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
