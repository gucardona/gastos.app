import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { bodyOf, mockFetch } from "../test/fixtures";
import { Login } from "./Login";

function renderLogin() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={["/login"]}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/" element={<p>Visão geral</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("Login", () => {
  it("entra e vai para a visão geral", async () => {
    const fetch = mockFetch({ body: { id: 1 } });
    const user = renderLogin();
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "senha-segura");
    await user.click(screen.getByRole("button", { name: "Entrar" }));
    expect(await screen.findByText("Visão geral")).toBeTruthy();
    expect(fetch.mock.calls[0][0]).toBe("/api/auth/login");
    expect(bodyOf(fetch)).toEqual({ email: "ana@example.com", password: "senha-segura" });
  });

  it("mostra a mensagem quando a senha está errada", async () => {
    mockFetch({ status: 401, body: { error: { code: "unauthenticated", message: "E-mail ou senha incorretos" } } });
    const user = renderLogin();
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "errada!!");
    await user.click(screen.getByRole("button", { name: "Entrar" }));
    expect(await screen.findByText("E-mail ou senha incorretos")).toBeTruthy();
    expect(screen.queryByText("Visão geral")).toBeNull();
  });

  it("cria a conta e mostra os erros de campo", async () => {
    const fetch = mockFetch({ status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { password: "mínimo de 8 caracteres" } } } });
    const user = renderLogin();
    await user.click(screen.getByRole("button", { name: "Criar uma conta" }));
    await user.type(screen.getByLabelText("Nome"), "Ana");
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "123");
    await user.click(screen.getByRole("button", { name: "Criar conta" }));
    expect(await screen.findByText("mínimo de 8 caracteres")).toBeTruthy();
    await waitFor(() => expect(fetch.mock.calls[0][0]).toBe("/api/auth/register"));
  });
});
