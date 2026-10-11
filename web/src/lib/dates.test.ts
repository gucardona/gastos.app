import { describe, expect, it } from "vitest";
import { addMonths, dayLabel, monthLabel, monthOf, monthRange, toISODate } from "./dates";

describe("month helpers", () => {
  it("limites do mês, inclusive bissexto", () => {
    expect(monthRange("2026-10")).toEqual({ from: "2026-10-01", to: "2026-10-31" });
    expect(monthRange("2028-02")).toEqual({ from: "2028-02-01", to: "2028-02-29" });
  });
  it("soma meses atravessando o ano", () => {
    expect(addMonths("2026-12", 1)).toBe("2027-01");
    expect(addMonths("2026-01", -1)).toBe("2025-12");
  });
  it("rótulo em português", () => {
    expect(monthLabel("2026-10")).toBe("outubro de 2026");
  });
  it("mês de uma data local, sem deslocar por fuso", () => {
    expect(toISODate(new Date(2026, 9, 31, 23, 59))).toBe("2026-10-31");
    expect(monthOf("2026-10-31")).toBe("2026-10");
  });
});

describe("dayLabel", () => {
  const today = "2026-10-11";
  it("hoje, ontem e dias anteriores", () => {
    expect(dayLabel("2026-10-11", today)).toBe("Hoje, 11 de outubro");
    expect(dayLabel("2026-10-10", today)).toBe("Ontem, 10 de outubro");
    expect(dayLabel("2026-10-09", today)).toBe("Sexta, 9 de outubro");
  });
});
