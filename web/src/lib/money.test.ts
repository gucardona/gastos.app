import { describe, expect, it } from "vitest";
import { formatBRL, parseCents, splitBRL } from "./money";

describe("parseCents", () => {
  it.each([
    ["12,90", 1290],
    ["1.234,56", 123456],
    ["12,9", 1290],
    ["0,05", 5],
    ["12", 1200],
    ["12.5", 1250],
    ["1.234", 123400],
    ["R$ 48,90", 4890],
    ["  7,00 ", 700],
  ])("%s → %i", (input, want) => expect(parseCents(input)).toBe(want));

  it.each(["", "abc", "12,345", "1,2,3", "-5", "0", "0,00"])("rejects %j", (input) =>
    expect(parseCents(input)).toBeNull(),
  );
});

describe("formatBRL", () => {
  it("formats cents as reais", () => {
    expect(formatBRL(123456).replace(/\s/g, " ")).toBe("R$ 1.234,56");
    expect(formatBRL(5).replace(/\s/g, " ")).toBe("R$ 0,05");
    expect(formatBRL(-2500).replace(/\s/g, " ")).toBe("-R$ 25,00");
  });
});

describe("splitBRL", () => {
  it("splits reais and cents", () => {
    expect(splitBRL(437660)).toEqual({ reais: "4.376", cents: "60" });
    expect(splitBRL(5)).toEqual({ reais: "0", cents: "05" });
    expect(splitBRL(-100)).toEqual({ reais: "-1", cents: "00" });
  });
});
