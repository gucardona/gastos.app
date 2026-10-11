import { describe, expect, it } from "vitest";
import { equalPercents, sharesFromPercents } from "./splits";

describe("sharesFromPercents", () => {
  it("converte porcentagens em basis points e exige soma 100%", () => {
    expect(sharesFromPercents({ 1: "70", 2: "30" })).toEqual([
      { userId: 1, shareBp: 7000 },
      { userId: 2, shareBp: 3000 },
    ]);
    expect(sharesFromPercents({ 1: "33,33", 2: "33,33", 3: "33,34" })).toHaveLength(3);
  });
  it("ignora quem ficou com 0 ou vazio", () => {
    expect(sharesFromPercents({ 1: "100", 2: "0", 3: "" })).toEqual([{ userId: 1, shareBp: 10000 }]);
  });
  it("devolve null quando a soma não fecha ou há valor inválido", () => {
    expect(sharesFromPercents({ 1: "60", 2: "30" })).toBeNull();
    expect(sharesFromPercents({ 1: "abc", 2: "100" })).toBeNull();
    expect(sharesFromPercents({})).toBeNull();
  });
});

describe("equalPercents", () => {
  it("divide igualmente e o resto fica com o primeiro", () => {
    expect(equalPercents([1, 2])).toEqual({ 1: "50", 2: "50" });
    expect(equalPercents([1, 2, 3])).toEqual({ 1: "33,34", 2: "33,33", 3: "33,33" });
  });
});
