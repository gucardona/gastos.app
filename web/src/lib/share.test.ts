import { describe, expect, it } from "vitest";
import { groupTop, pctLabel } from "./share";

describe("pctLabel", () => {
  it("rounds to an integer percent of the total", () => {
    expect(pctLabel(1, 3)).toBe("33%");
    expect(pctLabel(2, 3)).toBe("67%");
  });
  it("a single category is 100%", () => expect(pctLabel(500, 500)).toBe("100%"));
  it("shows <1% when above zero but rounding to zero", () => expect(pctLabel(1, 1000)).toBe("<1%"));
  it("is 0% for zero, and safe with a zero total", () => {
    expect(pctLabel(0, 1000)).toBe("0%");
    expect(pctLabel(0, 0)).toBe("0%");
  });
});

describe("groupTop", () => {
  const pts = (n: number) => Array.from({ length: n }, (_, i) => ({ label: `C${i}`, value: (i + 1) * 10 }));
  it("keeps everything up to 8 points, sorted by value", () => {
    const out = groupTop(pts(8), 7, "Outros");
    expect(out).toHaveLength(8);
    expect(out[0]).toEqual({ label: "C7", value: 80 });
  });
  it("keeps the top 7 and sums the rest into the last point", () => {
    const out = groupTop(pts(10), 7, "Outros");
    expect(out).toHaveLength(8);
    expect(out[7]).toEqual({ label: "Outros", value: 10 + 20 + 30 });
  });
});
