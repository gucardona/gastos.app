import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

// The folder layout is a dependency rule: app → pages → features → ui / api / lib / hooks.
// Each test below names one direction that must not appear, so a misplaced import fails with its file and target.
const src = __dirname;
const walk = (d: string): string[] =>
  readdirSync(d).flatMap((f) => (statSync(join(d, f)).isDirectory() ? walk(join(d, f)) : [join(d, f)]));
const sources = walk(src).filter((f) => /\.(ts|tsx)$/.test(f) && !/\.test\.tsx?$/.test(f) && !f.includes(`${join(src, "test")}`));

/** Every relative import of a source file, as a path relative to src/ with forward slashes. */
function imports(file: string): { target: string; typeOnly: boolean }[] {
  const text = readFileSync(file, "utf8");
  return [...text.matchAll(/^import (type )?[^"]*?"(\.{1,2}\/[^"]+)"/gm)].map(([, type, spec]) => ({
    target: relative(src, resolve(dirname(file), spec)).replaceAll("\\", "/"),
    typeOnly: Boolean(type),
  }));
}

const top = (path: string) => path.split("/")[0];
const feature = (path: string) => (path.startsWith("features/") ? path.split("/")[1] : null);

/** The imports that break `allowed`, formatted as "file → target" so a failure says where to look. */
function violations(from: (file: string) => boolean, allowed: (target: string, typeOnly: boolean, file: string) => boolean) {
  return sources
    .map((f) => relative(src, f).replaceAll("\\", "/"))
    .filter(from)
    .flatMap((file) => imports(join(src, file)).filter((i) => !allowed(i.target, i.typeOnly, file)).map((i) => `${file} → ${i.target}`));
}

describe("folder structure", () => {
  it("ui/ is the house kit: it imports only ui/, lib/ and API types", () => {
    expect(violations((f) => top(f) === "ui", (t, typeOnly) => ["ui", "lib"].includes(top(t)) || (t === "api/types" && typeOnly))).toEqual([]);
  });

  it("api/ and lib/ know nothing about screens or components", () => {
    expect(violations((f) => ["api", "lib"].includes(top(f)), (t) => ["api", "lib"].includes(top(t)))).toEqual([]);
  });

  it("a feature imports no page, no other feature, and from app/ only the shared context", () => {
    expect(
      violations(
        (f) => top(f) === "features",
        (t, _typeOnly, file) => !["pages"].includes(top(t)) && (top(t) !== "app" || t === "app/context") && (feature(t) === null || feature(t) === feature(file)),
      ),
    ).toEqual([]);
  });

  it("a page imports no other page, except Settings and its own sections", () => {
    expect(
      violations(
        (f) => top(f) === "pages",
        (t, _typeOnly, file) => top(t) !== "pages" || (file === "pages/Settings.tsx" && t.startsWith("pages/settings/")) || (file.startsWith("pages/settings/") && t.startsWith("pages/settings/")),
      ),
    ).toEqual([]);
  });

  it("hooks/ import no page and no feature", () => {
    expect(violations((f) => top(f) === "hooks", (t) => !["pages", "features"].includes(top(t)))).toEqual([]);
  });
});
