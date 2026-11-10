import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Ctx } from "../context";
import { appCtx, members, wallet } from "../test/fixtures";
import { SplitEditor } from "./SplitEditor";

const ctx = (shares: number[] = [0, 0]) => appCtx({ wallet: { ...wallet, splittingEnabled: true }, members: members.map((m, i) => ({ ...m, defaultShareBp: shares[i] })) });
const ui = (c: ReturnType<typeof ctx>, value: Record<number, string> | null, onChange = vi.fn()) => (
  <Ctx.Provider value={c}><SplitEditor value={value} onChange={onChange} paidBy={1} cents={10000} /></Ctx.Provider>
);

describe("SplitEditor", () => {
  it("the chip is 'Ajustar' when not editing", () => {
    render(ui(ctx(), null));
    expect(screen.getByRole("button", { name: "Ajustar" })).toBeTruthy();
  });

  it("going back is 'Igualar' when there are no default shares (the default is equal)", () => {
    render(ui(ctx(), { 1: "50", 2: "50" }));
    expect(screen.getByRole("button", { name: "Igualar" })).toBeTruthy();
  });

  it("going back is 'Padrão' when every default member has a share (the default is not equal)", () => {
    render(ui(ctx([6000, 4000]), { 1: "50", 2: "50" }));
    expect(screen.getByRole("button", { name: "Padrão" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Igualar" })).toBeNull();
  });

  it("drops typed percentages of people who are no longer members", () => {
    const onChange = vi.fn();
    render(ui(ctx(), { 1: "50", 2: "30", 3: "20" }, onChange));
    expect(onChange).toHaveBeenCalledWith({ 1: "50", 2: "30" });
  });

  it("does not touch the value when everyone is still a member", () => {
    const onChange = vi.fn();
    render(ui(ctx(), { 1: "50", 2: "50" }, onChange));
    expect(onChange).not.toHaveBeenCalled();
  });

  describe("live total while editing", () => {
    const total = () => screen.getByText(/^Total/);
    it("says 100% when the parts add up", () => {
      render(ui(ctx(), { 1: "60", 2: "40" }));
      expect(total().textContent).toBe("Total 100%");
      expect(total().getAttribute("aria-live")).toBe("polite");
      expect(total().className).toContain("muted");
    });
    it("says how much is missing, with up to 2 decimals in pt-BR", () => {
      render(ui(ctx(), { 1: "33,33", 2: "56,6" }));
      expect(total().textContent).toBe("Total 89,93%, faltam 10,07%");
      expect(total().className).toContain("over");
    });
    it("says how much it went over", () => {
      render(ui(ctx(), { 1: "70", 2: "40" }));
      expect(total().textContent).toBe("Total 110%, passou 10%");
    });
    it("an empty part counts as zero", () => {
      render(ui(ctx(), { 1: "90", 2: "" }));
      expect(total().textContent).toBe("Total 90%, faltam 10%");
    });
    it("is not shown while a part is not a number (the submit error covers it), nor when not editing", () => {
      const { unmount } = render(ui(ctx(), { 1: "abc", 2: "50" }));
      expect(screen.queryByText(/^Total/)).toBeNull();
      unmount();
      render(ui(ctx(), null));
      expect(screen.queryByText(/^Total/)).toBeNull();
    });
  });
});
