import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ConfirmButton } from "./ConfirmButton";

function setup() {
  const onConfirm = vi.fn();
  render(
    <>
      <ConfirmButton className="chip" label="Nova conversa" confirmLabel="Confirmar: apagar conversa" onConfirm={onConfirm} />
      <button type="button">outro</button>
    </>,
  );
  return { onConfirm, user: userEvent.setup() };
}

describe("ConfirmButton", () => {
  it("the first click only asks; the second confirms", async () => {
    const { onConfirm, user } = setup();
    await user.click(screen.getByRole("button", { name: "Nova conversa" }));
    expect(onConfirm).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Confirmar: apagar conversa" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("a double click does not confirm", async () => {
    const { onConfirm, user } = setup();
    await user.dblClick(screen.getByRole("button", { name: "Nova conversa" }));
    expect(onConfirm).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Confirmar: apagar conversa" })).toBeTruthy();
  });

  it("the question takes the focus, so the keyboard does not lose its place", async () => {
    const { user } = setup();
    await user.click(screen.getByRole("button", { name: "Nova conversa" }));
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Confirmar: apagar conversa" }));
  });

  it("leaving it (focus elsewhere) cancels", async () => {
    const { onConfirm, user } = setup();
    await user.click(screen.getByRole("button", { name: "Nova conversa" }));
    await user.click(screen.getByRole("button", { name: "outro" }));
    expect(screen.getByRole("button", { name: "Nova conversa" })).toBeTruthy();
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("Esc cancels", async () => {
    setup();
    fireEvent.click(screen.getByRole("button", { name: "Nova conversa" }));
    fireEvent.keyDown(screen.getByRole("button", { name: "Confirmar: apagar conversa" }), { key: "Escape" });
    expect(screen.getByRole("button", { name: "Nova conversa" })).toBeTruthy();
  });
});
