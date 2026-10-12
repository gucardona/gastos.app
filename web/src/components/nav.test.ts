import { describe, expect, it } from "vitest";
import { formatBadge, navItems } from "./nav";

describe("navItems", () => {
  it("lists the three everyday destinations when the wallet does not split", () => {
    expect(navItems(false).map((i) => i.label)).toEqual(["Visão", "Lançamentos", "Caixa"]);
  });

  it("adds Divisão, last, when the wallet splits expenses", () => {
    expect(navItems(true).map((i) => i.label)).toEqual(["Visão", "Lançamentos", "Caixa", "Divisão"]);
  });

  it("never links to Ajustes or Recorrentes: those have other paths", () => {
    const targets = navItems(true).map((i) => i.to);
    expect(targets).not.toContain("/ajustes");
    expect(targets).not.toContain("/recorrentes");
  });
});

describe("formatBadge", () => {
  it.each([
    [0, ""],
    [-1, ""],
    [4, "4"],
    [99, "99"],
    [100, "99+"],
    [250, "99+"],
  ])("%i shows %j", (n, text) => {
    expect(formatBadge(n)).toBe(text);
  });
});
