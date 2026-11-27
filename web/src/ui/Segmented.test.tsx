import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Segmented } from "./Segmented";

const options = [
  { value: "expense", label: "Gasto" },
  { value: "income", label: "Entrada" },
] as const;

describe("Segmented", () => {
  it("is a named group with the options in the order received", () => {
    render(<Segmented label="Tipo" value="expense" onChange={vi.fn()} options={[...options]} />);
    const group = screen.getByRole("group", { name: "Tipo" });
    expect([...group.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["Gasto", "Entrada"]);
  });

  it("marks only the current option as pressed", () => {
    render(<Segmented label="Tipo" value="income" onChange={vi.fn()} options={[...options]} />);
    expect(screen.getByRole("button", { name: "Gasto" }).getAttribute("aria-pressed")).toBe("false");
    expect(screen.getByRole("button", { name: "Entrada" }).getAttribute("aria-pressed")).toBe("true");
  });

  it("reports the value of the clicked option, including the one already active", async () => {
    const onChange = vi.fn();
    render(<Segmented label="Tipo" value="expense" onChange={onChange} options={[...options]} />);
    await userEvent.click(screen.getByRole("button", { name: "Entrada" }));
    expect(onChange).toHaveBeenLastCalledWith("income");
    await userEvent.click(screen.getByRole("button", { name: "Gasto" }));
    expect(onChange).toHaveBeenLastCalledWith("expense");
  });
});
