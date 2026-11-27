import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ColorSwatches, SWATCH_COLORS } from "./ColorSwatches";

const marked = () => screen.getAllByRole("button").filter((b) => b.getAttribute("aria-pressed") === "true").map((b) => b.getAttribute("aria-label"));

describe("ColorSwatches", () => {
  it("offers the 8 colors, with only the current one marked", () => {
    render(<ColorSwatches value="#3B6FD8" onPick={vi.fn()} />);
    expect(screen.getAllByRole("button")).toHaveLength(8);
    expect(SWATCH_COLORS).toHaveLength(8);
    expect(marked()).toEqual(["Cor #3B6FD8"]);
  });

  it("reports the hex tapped", async () => {
    const onPick = vi.fn();
    render(<ColorSwatches value="#3B6FD8" onPick={onPick} />);
    await userEvent.click(screen.getByRole("button", { name: "Cor #D4568B" }));
    expect(onPick).toHaveBeenCalledWith("#D4568B");
  });

  it("matches the current color regardless of letter case", () => {
    render(<ColorSwatches value="#3b6fd8" onPick={vi.fn()} />);
    expect(screen.getAllByRole("button")).toHaveLength(8);
    expect(marked()).toEqual(["Cor #3B6FD8"]);
  });

  it("a color from the old picker (outside the palette) is kept as a ninth, marked swatch", async () => {
    const onPick = vi.fn();
    render(<ColorSwatches value="#1f7d8f" onPick={onPick} />);
    expect(screen.getAllByRole("button")).toHaveLength(9);
    expect(marked()).toEqual(["Cor #1f7d8f"]);
    await userEvent.click(screen.getByRole("button", { name: "Cor #1f7d8f" }));
    expect(onPick).toHaveBeenCalledWith("#1f7d8f");
  });
});
