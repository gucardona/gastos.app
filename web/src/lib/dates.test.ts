import { afterEach, describe, expect, it, vi } from "vitest";
import { addDays, addMonths, dayLabel, monthLabel, monthOf, monthRange, toISODate, yesterday } from "./dates";

describe("month helpers", () => {
  it("month bounds, including leap years", () => {
    expect(monthRange("2026-10")).toEqual({ from: "2026-10-01", to: "2026-10-31" });
    expect(monthRange("2028-02")).toEqual({ from: "2028-02-01", to: "2028-02-29" });
  });
  it("adds months across the year boundary", () => {
    expect(addMonths("2026-12", 1)).toBe("2027-01");
    expect(addMonths("2026-01", -1)).toBe("2025-12");
  });
  it("label in Portuguese", () => {
    expect(monthLabel("2026-10")).toBe("outubro de 2026");
  });
  it("month of a local date, without shifting by time zone", () => {
    expect(toISODate(new Date(2026, 9, 31, 23, 59))).toBe("2026-10-31");
    expect(monthOf("2026-10-31")).toBe("2026-10");
  });
});

describe("addDays", () => {
  it("adds and subtracts days across month and year boundaries", () => {
    expect(addDays("2026-10-31", 1)).toBe("2026-11-01");
    expect(addDays("2026-01-01", -1)).toBe("2025-12-31");
    expect(addDays("2028-02-28", 1)).toBe("2028-02-29");
  });
});

describe("dayLabel", () => {
  const today = "2026-10-11";
  it("today, yesterday and earlier days", () => {
    expect(dayLabel("2026-10-11", today)).toBe("Hoje, 11 de outubro");
    expect(dayLabel("2026-10-10", today)).toBe("Ontem, 10 de outubro");
    expect(dayLabel("2026-10-09", today)).toBe("Sexta, 9 de outubro");
  });
});

describe("yesterday", () => {
  afterEach(() => vi.useRealTimers());

  it("is the day before today, across a month boundary", () => {
    vi.useFakeTimers({ now: new Date(2026, 2, 1, 12) }); // 1 March 2026
    expect(yesterday()).toBe("2026-02-28");
  });
});
