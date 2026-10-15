import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { ChartData } from "../api/types";
import { Chart } from "./Chart";

const points = [
  { label: "Alimentação", value: 82000 },
  { label: "Transporte", value: 120000 },
  { label: "Lazer", value: 60000 },
];
const make = (kind: ChartData["kind"], p = points): ChartData => ({ kind, title: "Gastos do mês", points: p });

describe("Chart", () => {
  it("bars: one bar per point, captioned and described", () => {
    const { container } = render(<Chart chart={make("bars")} />);
    expect(container.querySelectorAll("rect.chart-bar")).toHaveLength(3);
    expect(container.querySelector("figcaption")?.textContent).toBe("Gastos do mês");
    const label = screen.getByRole("img").getAttribute("aria-label")!;
    expect(label).toContain("Alimentação: R$");
    expect(label).toContain("820,00");
  });

  it("line: one polyline", () => {
    const { container } = render(<Chart chart={make("line")} />);
    expect(container.querySelectorAll("polyline")).toHaveLength(1);
  });

  it("pie: one slice per point with the percentage in the label", () => {
    const { container } = render(<Chart chart={make("pie")} />);
    expect(container.querySelectorAll(".chart-slice")).toHaveLength(3);
    expect(screen.getByRole("img").getAttribute("aria-label")).toMatch(/Alimentação: R\$\s820,00, 31%/);
  });

  it("pie: a single slice is a full circle", () => {
    const { container } = render(<Chart chart={make("pie", [points[0]])} />);
    expect(container.querySelectorAll("circle.chart-slice")).toHaveLength(1);
  });

  it("pie: zero slices are skipped and an all-zero pie does not break", () => {
    const { container, rerender } = render(<Chart chart={make("pie", [points[0], { label: "Zero", value: 0 }])} />);
    expect(container.querySelectorAll(".chart-slice")).toHaveLength(1);
    rerender(<Chart chart={make("pie", [{ label: "Zero", value: 0 }])} />);
    expect(container.querySelectorAll(".chart-slice")).toHaveLength(0);
    expect(screen.getByRole("img").getAttribute("aria-label")).toContain("Zero: R$");
  });

  it("empty points show Sem dados", () => {
    render(<Chart chart={make("bars", [])} />);
    expect(screen.getByText("Sem dados")).toBeTruthy();
    expect(screen.queryByRole("img")).toBeNull();
  });

  it("long labels are truncated", () => {
    const long = "x".repeat(60);
    const { container } = render(<Chart chart={make("bars", [{ label: long, value: 100 }])} />);
    const t = container.querySelector(".chart-label")!;
    expect(t.textContent!.length).toBeLessThan(60);
    expect(t.textContent).toMatch(/…$/);
  });
});
