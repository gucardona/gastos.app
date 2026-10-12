import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { bodyOf, mockFetch } from "../test/fixtures";
import { Login } from "./Login";

type Mode = "open" | "code" | "closed";
type Reply = { status?: number; body?: unknown };

/** Serves the registration mode, and `reply` for everything else (login and sign-up calls). */
function serve(mode: Mode | "error", reply: Reply = { body: { id: 1 } }) {
  return mockFetch((url) => {
    if (url !== "/api/auth/registration") return reply;
    return mode === "error" ? { status: 500, body: { error: { code: "internal", message: "Erro interno" } } } : { body: { mode } };
  });
}

function renderLogin() {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
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

const calls = (fetch: ReturnType<typeof mockFetch>, url: string) => fetch.mock.calls.filter((c) => c[0] === url);
const bodyTo = (fetch: ReturnType<typeof mockFetch>, url: string) => bodyOf(fetch, fetch.mock.calls.indexOf(calls(fetch, url)[0]));

describe("Login", () => {
  it("signs in and goes to the overview", async () => {
    const fetch = serve("code");
    const user = renderLogin();
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "senha-segura");
    await user.click(screen.getByRole("button", { name: "Entrar" }));
    expect(await screen.findByText("Visão geral")).toBeTruthy();
    expect(bodyTo(fetch, "/api/auth/login")).toEqual({ email: "ana@example.com", password: "senha-segura" });
  });

  it("shows the message when the password is wrong", async () => {
    serve("code", { status: 401, body: { error: { code: "unauthenticated", message: "E-mail ou senha incorretos" } } });
    const user = renderLogin();
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "errada!!");
    await user.click(screen.getByRole("button", { name: "Entrar" }));
    expect(await screen.findByText("E-mail ou senha incorretos")).toBeTruthy();
    expect(screen.queryByText("Visão geral")).toBeNull();
  });
});

describe("Login: sign-up by registration mode", () => {
  it("open (first account): sign-up needs no invite code", async () => {
    const fetch = serve("open");
    const user = renderLogin();
    await user.click(await screen.findByRole("button", { name: "Criar uma conta" }));
    expect(screen.queryByLabelText("Código de convite")).toBeNull();
    await user.type(screen.getByLabelText("Como você gostaria de ser chamado?"), "Ana");
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "senha-segura");
    await user.type(screen.getByLabelText("Repita a senha"), "senha-segura");
    await user.click(screen.getByRole("button", { name: "Criar conta" }));
    expect(await screen.findByText("Visão geral")).toBeTruthy();
    expect(bodyTo(fetch, "/api/auth/register")).toEqual({ name: "Ana", email: "ana@example.com", password: "senha-segura" });
  });

  it("code: asks for the invite code and sends it", async () => {
    const fetch = serve("code");
    const user = renderLogin();
    await user.click(await screen.findByRole("button", { name: "Criar uma conta" }));
    await user.type(screen.getByLabelText("Como você gostaria de ser chamado?"), "Bia");
    await user.type(screen.getByLabelText("E-mail"), "bia@example.com");
    await user.type(screen.getByLabelText("Senha"), "senha-segura");
    await user.type(screen.getByLabelText("Repita a senha"), "senha-segura");
    await user.type(screen.getByLabelText("Código de convite"), "open-sesame-42");
    await user.click(screen.getByRole("button", { name: "Criar conta" }));
    expect(await screen.findByText("Visão geral")).toBeTruthy();
    expect(bodyTo(fetch, "/api/auth/register")).toMatchObject({ name: "Bia", inviteCode: "open-sesame-42" });
  });

  it("code: the invite code field is not shown on the sign-in form", async () => {
    serve("code");
    renderLogin();
    await screen.findByRole("button", { name: "Criar uma conta" });
    expect(screen.queryByLabelText("Código de convite")).toBeNull();
  });

  it("code: a refused code shows the server's message", async () => {
    serve("code", { status: 403, body: { error: { code: "forbidden", message: "O cadastro está fechado. Peça um convite." } } });
    const user = renderLogin();
    await user.click(await screen.findByRole("button", { name: "Criar uma conta" }));
    await user.type(screen.getByLabelText("Como você gostaria de ser chamado?"), "Bia");
    await user.type(screen.getByLabelText("E-mail"), "bia@example.com");
    await user.type(screen.getByLabelText("Senha"), "senha-segura");
    await user.type(screen.getByLabelText("Repita a senha"), "senha-segura");
    await user.type(screen.getByLabelText("Código de convite"), "wrong-code-123");
    await user.click(screen.getByRole("button", { name: "Criar conta" }));
    expect(await screen.findByText("O cadastro está fechado. Peça um convite.")).toBeTruthy();
    expect(screen.queryByText("Visão geral")).toBeNull();
  });

  it("closed: there is no way to start a sign-up", async () => {
    const fetch = serve("closed");
    renderLogin();
    await waitFor(() => expect(calls(fetch, "/api/auth/registration")).toHaveLength(1));
    await waitFor(() => expect(screen.getByText(/cadastro está fechado/i)).toBeTruthy());
    expect(screen.queryByRole("button", { name: "Criar uma conta" })).toBeNull();
    expect(screen.getByRole("button", { name: "Entrar" })).toBeTruthy();
  });

  it("does not offer sign-up until the mode is known", () => {
    mockFetch(() => new Promise(() => {}) as never);
    renderLogin();
    expect(screen.queryByRole("button", { name: "Criar uma conta" })).toBeNull();
    expect(screen.getByRole("button", { name: "Entrar" })).toBeTruthy(); // signing in works meanwhile
  });

  it("if the mode cannot be loaded, still offers sign-up (the server enforces the rule)", async () => {
    serve("error");
    renderLogin();
    expect(await screen.findByRole("button", { name: "Criar uma conta" })).toBeTruthy();
  });

  it("creates the account and shows field errors", async () => {
    const fetch = serve("open", { status: 400, body: { error: { code: "validation", message: "Dados inválidos", fields: { password: "mínimo de 8 caracteres" } } } });
    const user = renderLogin();
    await user.click(await screen.findByRole("button", { name: "Criar uma conta" }));
    await user.type(screen.getByLabelText("Como você gostaria de ser chamado?"), "Ana");
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "123");
    await user.type(screen.getByLabelText("Repita a senha"), "123");
    await user.click(screen.getByRole("button", { name: "Criar conta" }));
    expect(await screen.findByText("mínimo de 8 caracteres")).toBeTruthy();
    await waitFor(() => expect(calls(fetch, "/api/auth/register")).toHaveLength(1));
  });

  it("refuses mismatched passwords without calling the server", async () => {
    const fetch = serve("open");
    const user = renderLogin();
    await user.click(await screen.findByRole("button", { name: "Criar uma conta" }));
    await user.type(screen.getByLabelText("Como você gostaria de ser chamado?"), "Ana");
    await user.type(screen.getByLabelText("E-mail"), "ana@example.com");
    await user.type(screen.getByLabelText("Senha"), "senha-segura");
    await user.type(screen.getByLabelText("Repita a senha"), "senha-segurA");
    await user.click(screen.getByRole("button", { name: "Criar conta" }));
    expect(await screen.findByText("As senhas não são iguais.")).toBeTruthy();
    expect(calls(fetch, "/api/auth/register")).toHaveLength(0);
  });

  it("does not ask to repeat the password when signing in", async () => {
    serve("open");
    renderLogin();
    await screen.findByRole("button", { name: "Criar uma conta" });
    expect(screen.queryByLabelText("Repita a senha")).toBeNull();
  });
});
