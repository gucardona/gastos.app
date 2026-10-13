import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ListGroup, ListRow } from "./ListGroup";
import { Switch } from "./Switch";

describe("ListRow", () => {
  it("with onClick it is a button whose name joins the label and the value", async () => {
    const onClick = vi.fn();
    render(<ListGroup><ListRow label="Categoria" value="Mercado" onClick={onClick} chevron /></ListGroup>);
    const row = screen.getByRole("button", { name: "Categoria Mercado" });
    await userEvent.click(row);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("the chevron is decorative: it does not enter the accessible name", () => {
    render(<ListRow label="Pessoas" onClick={vi.fn()} chevron />);
    expect(screen.getByRole("button", { name: "Pessoas" })).toBeTruthy();
  });

  it("without onClick it is not a button", () => {
    render(<ListRow label="Pago com" value="Pix" />);
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText("Pago com")).toBeTruthy();
    expect(screen.getByText("Pix")).toBeTruthy();
  });

  it("disabled turns the button off", () => {
    render(<ListRow label="Categoria" onClick={vi.fn()} disabled />);
    expect((screen.getByRole("button") as HTMLButtonElement).disabled).toBe(true);
  });

  it("holds a Switch as trailing content, and toggling it reports the new value", async () => {
    const onChange = vi.fn();
    render(<ListRow label="Repetir todo mês" trailing={<Switch checked={false} onChange={onChange} label="Repetir todo mês" />} />);
    await userEvent.click(screen.getByRole("switch", { name: "Repetir todo mês" }));
    expect(onChange).toHaveBeenCalledWith(true);
  });
});
