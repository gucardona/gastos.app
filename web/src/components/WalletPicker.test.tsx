import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { wallet } from "../test/fixtures";
import { WalletPicker } from "./WalletPicker";

const wallets = [wallet, { ...wallet, id: 2, name: "Viagem" }, { ...wallet, id: 3, name: "Empresa" }];

function Harness({ initial, onChange }: { initial: number[]; onChange?: (ids: number[]) => void }) {
  const [selected, setSelected] = useState(initial);
  return <WalletPicker wallets={wallets} selected={selected} onChange={(ids) => { setSelected(ids); onChange?.(ids); }} />;
}

const sheet = () => screen.getByRole("dialog", { name: "Carteiras consultadas" });

describe("WalletPicker", () => {
  it("shows the wallet's name with one selected, and how many with several", () => {
    const { unmount } = render(<Harness initial={[1]} />);
    expect(screen.getByRole("button", { name: /Casa/ })).toBeTruthy();
    unmount();
    render(<Harness initial={[1, 3]} />);
    expect(screen.getByRole("button", { name: /2 carteiras/ })).toBeTruthy();
  });

  it("opens a list of checkboxes and Selecionar todas checks every wallet", async () => {
    const onChange = vi.fn();
    render(<Harness initial={[1]} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /Casa/ }));
    expect(within(sheet()).getByRole("checkbox", { name: "Casa" })).toHaveProperty("checked", true);
    expect(within(sheet()).getByRole("checkbox", { name: "Viagem" })).toHaveProperty("checked", false);
    await userEvent.click(within(sheet()).getByRole("button", { name: "Selecionar todas" }));
    expect(onChange).toHaveBeenLastCalledWith([1, 2, 3]);
    expect(screen.getByRole("button", { name: /3 carteiras/ })).toBeTruthy();
  });

  it("never unchecks the last wallet", async () => {
    const onChange = vi.fn();
    render(<Harness initial={[1]} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /Casa/ }));
    const casa = within(sheet()).getByRole("checkbox", { name: "Casa" });
    expect(casa.getAttribute("aria-disabled")).toBe("true");
    expect(casa).toHaveProperty("disabled", false); // still reachable with Tab
    expect(document.getElementById(casa.getAttribute("aria-describedby") ?? "")?.textContent).toBe("Pelo menos uma carteira.");
    await userEvent.click(casa);
    expect(casa).toHaveProperty("checked", true);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("shows the first wallet's name when the selected one is not in the list", () => {
    render(<Harness initial={[99]} />);
    expect(screen.getByRole("button", { name: /Casa/ })).toBeTruthy();
  });

  it("works with the keyboard", async () => {
    const onChange = vi.fn();
    render(<Harness initial={[1]} onChange={onChange} />);
    screen.getByRole("button", { name: /Casa/ }).focus();
    await userEvent.keyboard("{Enter}");
    within(sheet()).getByRole("checkbox", { name: "Viagem" }).focus();
    await userEvent.keyboard(" ");
    expect(onChange).toHaveBeenLastCalledWith([1, 2]);
    await userEvent.click(within(sheet()).getByRole("button", { name: "Fechar" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("button", { name: /2 carteiras/ })).toBeTruthy();
  });
});
