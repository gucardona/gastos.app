import { describe, expect, it } from "vitest";
import { equalPercents, sharesFromPercents } from "./splits";

describe("sharesFromPercents", () => {
  it("converts percentages to basis points and requires a 100% total", () => {
    expect(sharesFromPercents({ 1: "70", 2: "30" })).toEqual([
      { userId: 1, shareBp: 7000 },
      { userId: 2, shareBp: 3000 },
    ]);
    expect(sharesFromPercents({ 1: "33,33", 2: "33,33", 3: "33,34" })).toHaveLength(3);
  });
  it("ignores whoever is left at 0 or empty", () => {
    expect(sharesFromPercents({ 1: "100", 2: "0", 3: "" })).toEqual([{ userId: 1, shareBp: 10000 }]);
  });
  it("returns null when the total does not add up or a value is invalid", () => {
    expect(sharesFromPercents({ 1: "60", 2: "30" })).toBeNull();
    expect(sharesFromPercents({ 1: "abc", 2: "100" })).toBeNull();
    expect(sharesFromPercents({})).toBeNull();
  });
});

describe("equalPercents", () => {
  it("splits equally and the remainder goes to the first", () => {
    expect(equalPercents([1, 2])).toEqual({ 1: "50", 2: "50" });
    expect(equalPercents([1, 2, 3])).toEqual({ 1: "33,34", 2: "33,33", 3: "33,33" });
  });
});
