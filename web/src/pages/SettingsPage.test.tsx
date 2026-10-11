import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it } from "vitest";
import { Ctx } from "../context";
import { appCtx, mockFetch } from "../test/fixtures";
import { Settings } from "./Settings";

function Where() {
  const l = useLocation();
  return <p data-testid="where">{l.pathname}{l.search}</p>;
}

function renderSettings(route = "/ajustes") {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route path="/ajustes" element={<Ctx.Provider value={appCtx()}><Settings /><Where /></Ctx.Provider>} />
          <Route path="/login" element={<p>tela de login</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

beforeEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
});

describe("Settings", () => {
  it("opens on the account, with name, email and the role in the wallet", () => {
    mockFetch();
    renderSettings();
    expect(screen.getByRole("button", { name: "Conta" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByText("ana@example.com")).toBeTruthy();
    expect(screen.getByText(/Casa, você administra/)).toBeTruthy();
  });

  it("switches section and keeps the choice in the URL", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderSettings();
    await user.click(screen.getByRole("button", { name: "Membros" }));
    expect(screen.getByTestId("where").textContent).toBe("/ajustes?s=membros");
    expect(screen.getByText("Convidar alguém")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Membros" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("button", { name: "Conta" }).getAttribute("aria-pressed")).toBe("false");
  });

  it.each([
    ["carteira", "Nova carteira"],
    ["categorias", "Nova categoria"],
    ["pagamentos", "Nova forma de pagamento"],
    ["orcamentos", "O limite mensal de cada categoria"],
  ])("a seção %s abre pelo link direto", async (section, marker) => {
    mockFetch({ body: [] });
    renderSettings(`/ajustes?s=${section}`);
    expect(await screen.findByText(new RegExp(marker))).toBeTruthy();
  });

  it("an unknown section falls back to the account", () => {
    mockFetch();
    renderSettings("/ajustes?s=nao-existe");
    expect(screen.getByText("ana@example.com")).toBeTruthy();
  });

  it("the chosen theme applies immediately and is remembered", async () => {
    mockFetch();
    const user = userEvent.setup();
    renderSettings();
    await user.selectOptions(screen.getByLabelText("Tema"), "dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem("theme")).toBe("dark");
    await user.selectOptions(screen.getByLabelText("Tema"), "system");
    expect(document.documentElement.dataset.theme).toBeUndefined();
    expect(localStorage.getItem("theme")).toBeNull();
  });

  it("signing out ends the session and returns to the login", async () => {
    const fetch = mockFetch({ status: 204 });
    const user = userEvent.setup();
    renderSettings();
    await user.click(screen.getByRole("button", { name: "Sair" }));
    expect(await screen.findByText("tela de login")).toBeTruthy();
    expect(fetch.mock.calls[0][0]).toBe("/api/auth/logout");
    expect(fetch.mock.calls[0][1]!.method).toBe("POST");
  });

  it("signing out works even if the call fails (offline)", async () => {
    const fetch = mockFetch();
    fetch.mockRejectedValue(new Error("offline"));
    const user = userEvent.setup();
    renderSettings();
    await user.click(screen.getByRole("button", { name: "Sair" }));
    await waitFor(() => expect(screen.getByText("tela de login")).toBeTruthy());
  });
});
