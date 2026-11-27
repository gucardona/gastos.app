import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Sheet } from "./Sheet";

const dialog = (container: HTMLElement) => container.querySelector("dialog") as HTMLDialogElement;

describe("Sheet", () => {
  it("renders nothing inside while closed", () => {
    const { container } = render(<Sheet open={false} onClose={vi.fn()} label="Novo lançamento"><p>conteúdo</p></Sheet>);
    expect(screen.queryByText("conteúdo")).toBeNull();
    expect(dialog(container).hasAttribute("open")).toBe(false);
  });

  it("opens as a named modal dialog with its content", () => {
    const { container } = render(<Sheet open onClose={vi.fn()} label="Novo lançamento"><p>conteúdo</p></Sheet>);
    expect(dialog(container).hasAttribute("open")).toBe(true);
    expect(dialog(container).getAttribute("aria-label")).toBe("Novo lançamento");
    expect(screen.getByText("conteúdo")).toBeTruthy();
  });

  it("reports closing when the browser closes it (Esc)", () => {
    const onClose = vi.fn();
    const { container } = render(<Sheet open onClose={onClose} label="x"><p>conteúdo</p></Sheet>);
    fireEvent(dialog(container), new Event("close"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does not close on a click outside it (the backdrop) or inside: only its own close button, or Esc, closes it", () => {
    const onClose = vi.fn();
    const { container } = render(<Sheet open onClose={onClose} label="x"><p>conteúdo</p></Sheet>);
    fireEvent.click(screen.getByText("conteúdo"));
    fireEvent.click(dialog(container)); // a click on the backdrop lands on the dialog element itself
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes the dialog when open goes from true to false", () => {
    const { container, rerender } = render(<Sheet open onClose={vi.fn()} label="x"><p>conteúdo</p></Sheet>);
    rerender(<Sheet open={false} onClose={vi.fn()} label="x"><p>conteúdo</p></Sheet>);
    expect(dialog(container).hasAttribute("open")).toBe(false);
    expect(screen.queryByText("conteúdo")).toBeNull();
  });
});
