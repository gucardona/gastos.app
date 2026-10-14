import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(join(__dirname, "..", "styles.css"), "utf8");

describe("keyboard focus stays visible", () => {
  it("inputs that hide the browser outline draw a ring on their row instead", () => {
    // .desc-input removes its own outline; the row around it must show the focus
    expect(css).toMatch(/\.desc-input\s*\{[^}]*outline:\s*none/);
    expect(css).toMatch(/\.list-row:focus-within[^{]*\{[^}]*(box-shadow|outline)/);
    expect(css).toMatch(/\.field-row:focus-within[^{]*\{[^}]*(box-shadow|outline)/);
  });
});
