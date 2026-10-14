import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(join(__dirname, "..", "styles.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, ""); // comments are not rules
const rules = [...css.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ selector: m[1].trim(), body: m[2] }));

// where the design system allows centered text: empty states, the icon preview, the amount of the quick-add sheet,
// the category tiles, the title of a picker, the emoji grid, the day numbers under the ruler ticks and an icon in its fixed column
const CENTERED_OK = [".empty", ".preview", ".amount", ".cat-tile", ".picker-head", ".emoji-grid", ".ruler-axis", ".row .emoji"];

describe("alignment rule", () => {
  it("text is centered only where the spec allows it", () => {
    const offenders = rules
      .filter((r) => /text-align:\s*center/.test(r.body))
      .map((r) => r.selector)
      .filter((sel) => !sel.split(",").every((one) => CENTERED_OK.some((ok) => one.trim().startsWith(ok))));
    expect(offenders).toEqual([]);
  });
});

describe("the old system is gone", () => {
  const OLD = ["item", "item-actions", "inline-form", "inline", "line", "settings", "text", "field", "seg"];

  it("no rule targets a class of the old forms", () => {
    const found = rules.flatMap((r) => OLD.filter((c) => new RegExp(`\\.${c}(?![-\\w])`).test(r.selector)).map((c) => `.${c} in "${r.selector}"`));
    expect(found).toEqual([]);
  });

  it("the --display variable is no longer used", () => {
    expect(css).not.toContain("--display");
  });
});
