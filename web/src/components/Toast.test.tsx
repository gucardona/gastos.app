import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Sheet } from "./Sheet";
import { type ToastOptions, ToastProvider, useToast } from "./Toast";

function Trigger({ message, opts }: { message: string; opts?: ToastOptions }) {
  const toast = useToast();
  return <button type="button" onClick={() => toast(message, opts)}>disparar {message}</button>;
}

const fire = (message: string) => fireEvent.click(screen.getByRole("button", { name: `disparar ${message}` }));
const region = () => screen.getByRole("status");

describe("toast", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("shows the message in a polite live region that exists before any message", () => {
    render(<ToastProvider><Trigger message="Lançamento salvo" /></ToastProvider>);
    expect(region().getAttribute("aria-live")).toBe("polite");
    expect(region().textContent).toBe("");
    fire("Lançamento salvo");
    expect(region().textContent).toContain("Lançamento salvo");
  });

  it("goes away by itself after about 4 seconds", () => {
    render(<ToastProvider><Trigger message="Lançamento salvo" /></ToastProvider>);
    fire("Lançamento salvo");
    act(() => vi.advanceTimersByTime(3900));
    expect(screen.queryByText("Lançamento salvo")).toBeTruthy();
    act(() => vi.advanceTimersByTime(200));
    expect(screen.queryByText("Lançamento salvo")).toBeNull();
  });

  it("with an action it stays 8 seconds, and the action runs once and closes it", () => {
    const run = vi.fn();
    render(<ToastProvider><Trigger message="Lançamento excluído" opts={{ action: { label: "Desfazer", run } }} /></ToastProvider>);
    fire("Lançamento excluído");
    act(() => vi.advanceTimersByTime(7000));
    fireEvent.click(screen.getByRole("button", { name: "Desfazer" }));
    expect(run).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("Lançamento excluído")).toBeNull();
  });

  it("an action toast left alone goes away after 8 seconds", () => {
    render(<ToastProvider><Trigger message="Lançamento excluído" opts={{ action: { label: "Desfazer", run: vi.fn() } }} /></ToastProvider>);
    fire("Lançamento excluído");
    act(() => vi.advanceTimersByTime(8100));
    expect(screen.queryByText("Lançamento excluído")).toBeNull();
  });

  it("pauses while hovered or focused", () => {
    render(<ToastProvider><Trigger message="Lançamento salvo" /></ToastProvider>);
    fire("Lançamento salvo");
    const toast = screen.getByText("Lançamento salvo").closest(".toast")!;
    fireEvent.mouseEnter(toast);
    act(() => vi.advanceTimersByTime(10_000));
    expect(screen.queryByText("Lançamento salvo")).toBeTruthy();
    fireEvent.mouseLeave(toast);
    fireEvent.focus(screen.getByRole("button", { name: "Dispensar aviso" }));
    act(() => vi.advanceTimersByTime(10_000));
    expect(screen.queryByText("Lançamento salvo")).toBeTruthy();
    fireEvent.blur(screen.getByRole("button", { name: "Dispensar aviso" }));
    act(() => vi.advanceTimersByTime(4100));
    expect(screen.queryByText("Lançamento salvo")).toBeNull();
  });

  it("can be dismissed", () => {
    render(<ToastProvider><Trigger message="Lançamento salvo" /></ToastProvider>);
    fire("Lançamento salvo");
    fireEvent.click(screen.getByRole("button", { name: "Dispensar aviso" }));
    expect(screen.queryByText("Lançamento salvo")).toBeNull();
  });

  it("shows one message at a time: a new one replaces the old", () => {
    render(<ToastProvider><Trigger message="Primeiro" /><Trigger message="Segundo" /></ToastProvider>);
    fire("Primeiro");
    fire("Segundo");
    expect(screen.queryByText("Primeiro")).toBeNull();
    expect(screen.getByText("Segundo")).toBeTruthy();
  });

  it("inside an open sheet it shows there too, where it is not behind the modal", () => {
    const { container } = render(
      <ToastProvider>
        <Sheet open onClose={vi.fn()} label="x"><Trigger message="Lançamento salvo" /></Sheet>
      </ToastProvider>,
    );
    fire("Lançamento salvo");
    expect(container.querySelector("dialog .toast")?.textContent).toContain("Lançamento salvo");
  });

  it("without a provider, useToast is a harmless no-op", () => {
    render(<Trigger message="nada" />);
    expect(() => fire("nada")).not.toThrow();
  });
});
