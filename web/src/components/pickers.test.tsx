import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Category } from "../api/types";
import { addDays, today, yesterday } from "../lib/dates";
import { categories } from "../test/fixtures";
import { CategoryGrid, DatePicker, OptionList, PickerView } from "./pickers";

describe("PickerView", () => {
  it("shows its title and goes back", async () => {
    const onBack = vi.fn();
    render(<PickerView title="Categoria" onBack={onBack}><p>conteúdo</p></PickerView>);
    expect(screen.getByRole("heading", { name: "Categoria" })).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Voltar" }));
    expect(onBack).toHaveBeenCalledTimes(1);
  });
});

describe("CategoryGrid", () => {
  const expense = categories.filter((c) => c.kind === "expense");

  it("marks only the chosen category and reports the id picked", async () => {
    const onPick = vi.fn();
    render(<CategoryGrid categories={expense} value={2} onPick={onPick} />);
    expect(screen.getByRole("button", { name: /Restaurante/ }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("button", { name: /Mercado/ }).getAttribute("aria-pressed")).toBe("false");
    await userEvent.click(screen.getByRole("button", { name: /Mercado/ }));
    expect(onPick).toHaveBeenCalledWith(1);
  });

  it("lists every category it is given, even 25", () => {
    const many: Category[] = Array.from({ length: 25 }, (_, i) => ({ ...expense[0], id: 100 + i, name: `Categoria ${i}` }));
    render(<CategoryGrid categories={many} value={null} onPick={vi.fn()} />);
    expect(screen.getAllByRole("button")).toHaveLength(25);
  });

  it("an empty list does not break", () => {
    const { container } = render(<CategoryGrid categories={[]} value={null} onPick={vi.fn()} />);
    expect(container.querySelectorAll("button")).toHaveLength(0);
  });
});

describe("OptionList", () => {
  it("marks the chosen option and reports typed values (numbers and strings)", async () => {
    const onPick = vi.fn();
    const { unmount } = render(<OptionList options={[{ value: 1, label: "Pix" }, { value: 2, label: "Crédito" }]} value={2} onPick={onPick} />);
    expect(screen.getByRole("button", { name: "Crédito" }).getAttribute("aria-pressed")).toBe("true");
    await userEvent.click(screen.getByRole("button", { name: "Pix" }));
    expect(onPick).toHaveBeenLastCalledWith(1);
    unmount();
    render(<OptionList options={[{ value: "a", label: "Ana" }]} value={null} onPick={onPick} />);
    await userEvent.click(screen.getByRole("button", { name: "Ana" }));
    expect(onPick).toHaveBeenLastCalledWith("a");
  });
});

describe("DatePicker", () => {
  const pressed = () => screen.getAllByRole("button").filter((b) => b.getAttribute("aria-pressed") === "true").map((b) => b.querySelector(".lr-label")?.textContent);

  it("marks Hoje for today and Ontem for yesterday", () => {
    const { unmount } = render(<DatePicker value={today()} onPick={vi.fn()} />);
    expect(pressed()).toEqual(["Hoje"]);
    unmount();
    render(<DatePicker value={yesterday()} onPick={vi.fn()} />);
    expect(pressed()).toEqual(["Ontem"]);
  });

  it("marks neither for another day, and shows that day in the date field", () => {
    const other = addDays(today(), -9);
    render(<DatePicker value={other} onPick={vi.fn()} />);
    expect(pressed()).toEqual([]);
    expect((screen.getByLabelText("Data do lançamento") as HTMLInputElement).value).toBe(other);
  });

  it("picking Ontem reports yesterday's date, and the field reports the typed date", async () => {
    const onPick = vi.fn();
    render(<DatePicker value={today()} onPick={onPick} />);
    await userEvent.click(screen.getByRole("button", { name: "Ontem" }));
    expect(onPick).toHaveBeenLastCalledWith(yesterday());
    fireEvent.change(screen.getByLabelText("Data do lançamento"), { target: { value: "2026-01-15" } });
    expect(onPick).toHaveBeenLastCalledWith("2026-01-15");
  });
});
