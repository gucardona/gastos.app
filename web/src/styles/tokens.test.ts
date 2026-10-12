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
