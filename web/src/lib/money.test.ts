import { describe, expect, it } from "vitest";
import { amountProblem, centsToInput, formatBRL, parseCents, splitBRL, typedAmount } from "./money";

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

describe("centsToInput", () => {
  it("writes cents the way they are typed", () => {
    expect(centsToInput(12990)).toBe("129,90");
    expect(centsToInput(5)).toBe("0,05");
    expect(centsToInput(100000)).toBe("1.000,00");
  });
});

describe("amountProblem", () => {
  it("is empty when the amount can be saved", () => {
    expect(amountProblem("12,90")).toBe("");
    expect(amountProblem("1.234,56")).toBe("");
  });
  it("asks for at most 2 decimals when the text is a number with more", () => {
    expect(amountProblem("12,345")).toBe("Use até 2 casas decimais.");
    expect(amountProblem("R$ 1.234,567")).toBe("Use até 2 casas decimais.");
  });
  it("asks for an amount above zero otherwise", () => {
    expect(amountProblem("")).toBe("Informe um valor maior que zero.");
    expect(amountProblem("0,00")).toBe("Informe um valor maior que zero.");
    expect(amountProblem("abc")).toBe("Informe um valor maior que zero.");
  });
});

describe("typedAmount: digits fill from the cents, like a card machine", () => {
  it("each digit pushes the others to the left of the comma", () => {
    expect(typedAmount("5")).toBe("0,05");
    expect(typedAmount("58")).toBe("0,58");
    expect(typedAmount("587")).toBe("5,87");
    expect(typedAmount("58700")).toBe("587,00");
    expect(typedAmount("123456")).toBe("1.234,56");
  });

  it("works on what the field holds after a keystroke: the old mask plus the new digit, or minus one", () => {
    expect(typedAmount("5,877")).toBe("58,77"); // typed 7 after "5,87"
    expect(typedAmount("5,8")).toBe("0,58"); // backspace on "5,87"
    expect(typedAmount("1.234,567")).toBe("12.345,67");
  });

  it("empty, only zeros or no digits gives an empty field", () => {
    expect(typedAmount("")).toBe("");
    expect(typedAmount("0")).toBe("");
    expect(typedAmount("0,0")).toBe("");
    expect(typedAmount("R$ ")).toBe("");
  });

  it("ignores anything that is not a digit and caps at 12 digits (R$ 9.999.999.999,99)", () => {
    expect(typedAmount("R$ 12a3")).toBe("1,23");
    expect(typedAmount("1234567890123")).toBe(typedAmount("123456789012")); // the 13th digit is ignored
    expect(typedAmount("123456789012")).toBe("1.234.567.890,12");
  });

  it("what it produces always parses back to the same cents", () => {
    for (const d of ["5", "587", "58700", "123456", "100000000000"]) expect(parseCents(typedAmount(d))).toBe(Number(d));
  });

  it("an amount already saved shows the same way it would be typed", () => {
    expect(centsToInput(587)).toBe("5,87");
    expect(centsToInput(123456)).toBe("1.234,56");
  });
});
