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
});
