import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(join(__dirname, "tokens.css"), "utf8");

/** Body of the first rule whose selector is exactly `selector`. */
function block(selector: string): string {
  const start = css.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`no rule for ${selector}`);
  const open = css.indexOf("{", start);
  return css.slice(open + 1, css.indexOf("}", open));
}

const light = block(":root");
const darkBySystem = block(':root:not([data-theme="light"])');
const darkForced = block(':root[data-theme="dark"]');

const roles = ["paper", "surface", "ink", "muted", "line", "accent", "on-accent", "income", "over", "scrim", "tick", "tick-future"];

const value = (body: string, name: string) => new RegExp(`--${name}:\\s*([^;]+);`, "i").exec(body)?.[1].trim();

describe("design tokens", () => {
  for (const [name, body] of [["light", light], ["dark by system", darkBySystem], ["dark forced", darkForced]] as const) {
    it(`${name} defines every color role`, () => {
      for (const role of roles) expect(value(body, role), `--${role}`).toBeTruthy();
    });
  }

  it("uses the approved light palette", () => {
    expect(value(light, "paper")).toBe("#F2F3F4");
    expect(value(light, "accent")).toBe("#0E6577");
    expect(value(light, "over")).toBe("#C2412D");
  });

  it("uses the approved dark palette", () => {
    for (const body of [darkBySystem, darkForced]) {
      expect(value(body, "paper")).toBe("#0F1113");
      expect(value(body, "accent")).toBe("#6CC4D6");
      expect(value(body, "over")).toBe("#FF8A78");
    }
  });

  it("the system dark theme and the forced dark theme are identical", () => {
    for (const role of roles) expect(value(darkBySystem, role)).toBe(value(darkForced, role));
  });

  it("declares the typeface and the radii", () => {
    expect(value(light, "font")).toContain("Geist Variable");
    expect(value(light, "radius-group")).toBe("22px");
    expect(value(light, "radius-control")).toBe("12px");
  });
});

function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
const contrast = (a: string, b: string) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};

describe("contrast (WCAG AA, 4.5:1 for normal text)", () => {
  for (const [name, body] of [["light", light], ["dark", darkForced]] as const) {
    it(`${name}: secondary text is readable on the page and on cards`, () => {
      expect(contrast(value(body, "muted")!, value(body, "paper")!)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(value(body, "muted")!, value(body, "surface")!)).toBeGreaterThanOrEqual(4.5);
    });
    it(`${name}: the accent reads on the page and on cards`, () => {
      expect(contrast(value(body, "accent")!, value(body, "paper")!)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(value(body, "accent")!, value(body, "surface")!)).toBeGreaterThanOrEqual(4.5);
    });
  }
});

describe("contrast of the money colors", () => {
  for (const [name, body] of [["light", light], ["dark", darkForced]] as const) {
    it(`${name}: income green and the debt color read on cards and on the page`, () => {
      for (const role of ["income", "over"]) {
        expect(contrast(value(body, role)!, value(body, "surface")!), `${role} on surface`).toBeGreaterThanOrEqual(4.5);
        expect(contrast(value(body, role)!, value(body, "paper")!), `${role} on paper`).toBeGreaterThanOrEqual(4.5);
      }
    });
  }
});
