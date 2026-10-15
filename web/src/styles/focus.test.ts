import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(join(__dirname, "..", "styles.css"), "utf8");

describe("keyboard focus stays visible, and fits the shape it is on", () => {
  it("inputs that hide the browser outline draw a ring on their row, only while the input itself has the focus", () => {
    // .desc-input removes its own outline; the row around it must show the focus
    expect(css).toMatch(/\.desc-input\s*\{[^}]*outline:\s*none/);
    expect(css).toMatch(/\.list-row:has\(input:focus-visible\)[^{]*\{[^}]*outline:[^}]*outline-offset:\s*4px/);
    expect(css).toMatch(/\.field-row:has\(input:focus-visible\)[^{]*\{[^}]*box-shadow/);
  });

  it("a row that is itself focused (a link or a button) gets a rounded outline with room around the text, not a square box over it", () => {
    expect(css).toMatch(/\.list-row:focus-visible\s*\{[^}]*outline-offset:\s*4px[^}]*border-radius:\s*var\(--radius-control\)/);
  });

  it("no rule gives a focused row a square inset ring (focus-within matches the row itself too)", () => {
    expect(css).not.toMatch(/\.list-row:focus-within/);
  });
});
