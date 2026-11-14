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

  it("the group has enough padding above and below for the ring (offset 4px + width 2px = 6px, plus 10px of air, the same as on the sides), so it never touches the edge of the block", () => {
    const pad = css.match(/\.list-group\s*\{[^}]*padding:\s*(\d+)px/);
    expect(Number(pad?.[1])).toBeGreaterThanOrEqual(16);
  });

  it("no rule gives a focused row a square inset ring (focus-within matches the row itself too)", () => {
    expect(css).not.toMatch(/\.list-row:focus-within/);
  });
});

describe("the dividers next to a focused row", () => {
  it("are hidden, so they do not cut through the ring", () => {
    expect(css).toMatch(/\.list-row:is\(:focus-visible, :has\(input:focus-visible\)\) \+ \.list-row\s*\{[^}]*border-top-color:\s*transparent/);
  });

  it("are hidden around a focused transaction row too, which gets the same rounded ring", () => {
    expect(css).toMatch(/\.row:focus-visible\s*\{[^}]*outline-offset:\s*4px[^}]*border-radius:\s*var\(--radius-control\)/);
    expect(css).toMatch(/\.row:focus-visible, \.row:focus-visible \+ \.row\s*\{[^}]*border-top-color:\s*transparent/);
  });
});

describe("rings that used to be missing or clipped", () => {
  it("the big amount field, which hides its outline, shows an accent underline while focused", () => {
    expect(css).toMatch(/\.amount input\s*\{[^}]*outline:\s*none/);
    expect(css).toMatch(/\.amount input:focus-visible\s*\{[^}]*box-shadow:[^}]*var\(--accent\)/);
  });

  it("a segment's ring stays inside the 3px track (offset 0 + width 2px)", () => {
    expect(css).toMatch(/\.segmented\s*\{[^}]*padding:\s*3px/);
    expect(css).toMatch(/\.segmented button:focus-visible\s*\{[^}]*outline-offset:\s*0/);
  });

  for (const box of [".category-grid", ".emoji-tabs"]) {
    it(`${box} scrolls, so it pads 4px (offset 2px + width 2px) for the rings of the items at its edges`, () => {
      const body = new RegExp(`${box.replace(".", "\\.")}\\s*\\{([^}]*)\\}`).exec(css)?.[1] ?? "";
      expect(body).toMatch(/overflow-[xy]:\s*auto/);
      expect(body).toMatch(/(^|;)\s*padding:\s*4px/);
      expect(body).toMatch(/scroll-padding:\s*4px/);
    });
  }
});

describe("a stacked row (a long label on its own line, above a full-width input)", () => {
  const rules = [...css.replace(/\/\*[\s\S]*?\*\//g, "").matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ selector: m[1].trim(), body: m[2] }));
  const stacked = rules.filter((r) => /\.stacked(?![-\w])/.test(r.selector));

  it("wraps the label to a line of its own, and the input fills the next one with its text to the left", () => {
    expect(css).toMatch(/\.stacked\s*\{[^}]*flex-wrap:\s*wrap/);
    expect(css).toMatch(/\.stacked > \.lr-label\s*\{[^}]*flex:\s*1 0 100%/);
    expect(css).toMatch(/\.stacked \.desc-input\s*\{[^}]*text-align:\s*left/);
  });

  it("is a modifier of the same rows, so it keeps their ring and dividers: no stacked rule touches outline, box-shadow or borders", () => {
    expect(stacked.length).toBeGreaterThan(0);
    for (const r of stacked) expect(r.body, r.selector).not.toMatch(/outline|box-shadow|border/);
  });
});

describe("a date field inside a row", () => {
  it("does not draw its own ring inside the ring of its row: one ring, not two", () => {
    expect(css).toMatch(/\.date-input:focus-visible\s*\{[^}]*outline:\s*none/);
  });
});
