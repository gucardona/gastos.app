import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { bodyOf, mockFetch, renderApp } from "../test/fixtures";
import { PasswordSheet } from "./PasswordSheet";

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");

function open() {
  const onClose = vi.fn();
  const { toast } = renderApp(<PasswordSheet open onClose={onClose} />);
  return { onClose, toast, user: userEvent.setup() };
}

async function fill(user: ReturnType<typeof userEvent.setup>, current: string, next: string, again = next) {
  await user.type(screen.getByLabelText("Senha atual"), current);
  await user.type(screen.getByLabelText("Nova senha"), next);
  await user.type(screen.getByLabelText("Repetir a nova senha"), again);
}

describe("PasswordSheet", () => {
  it("asks for the three passwords as password fields", () => {
    open();
    for (const [name, auto] of [["Senha atual", "current-password"], ["Nova senha", "new-password"], ["Repetir a nova senha", "new-password"]]) {
      const input = screen.getByLabelText(name) as HTMLInputElement;
      expect(input.type).toBe("password");
      expect(input.autocomplete).toBe(auto);
    }
  });

  it("changes the password, then closes and says so in a toast", async () => {
    const fetch = mockFetch({ status: 204 });
    const { user, onClose, toast } = open();
    await fill(user, "provisoria-123", "minha-senha-nova");
    await user.click(screen.getByRole("button", { name: "Alterar senha" }));
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Senha alterada"));
    expect(onClose).toHaveBeenCalled();
    expect(writes(fetch)[0][0]).toBe("/api/me/password");
    expect(writes(fetch)[0][1]!.method).toBe("PUT");
    expect(bodyOf(fetch)).toEqual({ currentPassword: "provisoria-123", newPassword: "minha-senha-nova" });
  });

  it("does not send when the two new passwords differ", async () => {
    const fetch = mockFetch({ status: 204 });
    const { user, onClose } = open();
    await fill(user, "provisoria-123", "minha-senha-nova", "outra-coisa-123");
    await user.click(screen.getByRole("button", { name: "Alterar senha" }));
    expect(await screen.findByText("As senhas novas não coincidem.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("does not send a new password under 8 characters", async () => {
    const fetch = mockFetch({ status: 204 });
    const { user } = open();
    await fill(user, "provisoria-123", "curta");
    await user.click(screen.getByRole("button", { name: "Alterar senha" }));
    expect(await screen.findByText("A nova senha precisa ter 8 ou mais caracteres.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("shows the API's message when the current password is wrong, and stays open", async () => {
    mockFetch({ status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { currentPassword: "senha atual incorreta" } } } });
    const { user, onClose, toast } = open();
    await fill(user, "errada-errada", "minha-senha-nova");
    await user.click(screen.getByRole("button", { name: "Alterar senha" }));
    expect(await screen.findByText("senha atual incorreta")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
    expect(toast).not.toHaveBeenCalled();
  });

  it("shows the rate limit message", async () => {
    mockFetch({ status: 429, body: { error: { code: "rate_limited", message: "Muitas tentativas, aguarde alguns minutos" } } });
    const { user } = open();
    await fill(user, "x-x-x-x-x-x", "minha-senha-nova");
    await user.click(screen.getByRole("button", { name: "Alterar senha" }));
    expect(await screen.findByText("Muitas tentativas, aguarde alguns minutos")).toBeTruthy();
  });
});
