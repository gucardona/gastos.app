import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
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
          <Route path="/ajustes/:section?" element={<Ctx.Provider value={appCtx()}><Settings /><Where /></Ctx.Provider>} />
          <Route path="/login" element={<p>tela de login</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

const index = () => screen.getByRole("navigation", { name: "Ajustes" });

beforeEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
});

describe("Settings: the index", () => {
  it("lists the subjects in three groups, in order, each one a link to its own screen", () => {
    mockFetch();
    renderSettings();
    const links = within(index()).getAllByRole("link");
    expect(links.map((l) => l.getAttribute("href"))).toEqual([
      "/ajustes/perfil", "/ajustes/carteira", "/ajustes/pessoas", "/ajustes/categorias", "/ajustes/pagamentos", "/ajustes/orcamentos",
      "/ajustes/bancos", "/ajustes/recorrentes", "/ajustes/aparencia",
    ]);
    expect(links.map((l) => l.querySelector(".lr-label")?.textContent)).toEqual([
      "Perfil", "Carteira", "Pessoas", "Categorias", "Formas de pagamento", "Orçamentos", "Contas bancárias", "Recorrentes", "Aparência",
    ]);
    expect(screen.getByRole("heading", { name: "Casa" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Dados" })).toBeTruthy();
  });

  it("shows who you are on the Perfil row, and Sair last", () => {
    mockFetch();
    renderSettings();
    expect(within(index()).getByRole("link", { name: /Perfil/ }).textContent).toContain("Ana Souza");
    const all = within(index()).getAllByRole("link").length;
    expect(all).toBe(9);
    expect(within(index()).getAllByRole("button").at(-1)?.textContent).toBe("Sair");
  });

  it("marks the open subject", () => {
    mockFetch({ body: [] });
    renderSettings("/ajustes/categorias");
    const current = within(index()).getAllByRole("link").filter((l) => l.getAttribute("aria-current") === "page");
    expect(current.map((l) => l.getAttribute("href"))).toEqual(["/ajustes/categorias"]);
  });
});

describe("Settings: a screen per subject", () => {
  it("Perfil shows name, email and the role in the wallet", () => {
    mockFetch();
    renderSettings("/ajustes/perfil");
    expect(screen.getAllByText("ana@example.com").length).toBeGreaterThan(0);
    expect(screen.getByText(/Casa, você administra/)).toBeTruthy();
  });

  it("every screen has a way back to the index", () => {
    mockFetch({ body: [] });
    renderSettings("/ajustes/pessoas");
    expect(screen.getAllByRole("link").some((l) => l.getAttribute("href") === "/ajustes" && /Ajustes/.test(l.textContent ?? ""))).toBe(true);
  });

  it.each([
    ["carteira", /Nova carteira/],
    ["pessoas", /Convidar alguém/],
    ["categorias", /Nova categoria/],
    ["pagamentos", /Nova forma de pagamento/],
    ["orcamentos", /O limite mensal de cada categoria/],
  ])("%s opens by its direct link", async (section, marker) => {
    mockFetch({ body: [] });
    renderSettings(`/ajustes/${section}`);
    expect(await screen.findByText(marker)).toBeTruthy();
  });

  it("Recorrentes shows the monthly repeats", async () => {
    mockFetch({ body: [] });
    renderSettings("/ajustes/recorrentes");
    expect(await screen.findByRole("heading", { name: "Repetem todo mês" })).toBeTruthy();
  });
});

describe("Settings: old addresses and unknown sections", () => {
  it.each([
    ["bancos", "/ajustes/bancos"],
    ["conta", "/ajustes/perfil"],
    ["membros", "/ajustes/pessoas"],
    ["categorias", "/ajustes/categorias"],
  ])("?s=%s goes to %s", async (old, now) => {
    mockFetch({ body: [] });
    renderSettings(`/ajustes?s=${old}`);
    await waitFor(() => expect(screen.getByTestId("where").textContent).toBe(now));
  });

  it("an unknown section, old or new, goes back to the index", async () => {
    mockFetch({ body: [] });
    renderSettings("/ajustes/nao-existe");
    await waitFor(() => expect(screen.getByTestId("where").textContent).toBe("/ajustes"));
  });
});

describe("Settings: Aparência and Sair", () => {
  it("the chosen theme applies immediately and is remembered", async () => {
    mockFetch();
    const user = renderSettings("/ajustes/aparencia");
    await user.click(screen.getByRole("button", { name: "Escuro" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem("theme")).toBe("dark");
    expect(screen.getByRole("button", { name: "Escuro" }).getAttribute("aria-pressed")).toBe("true");
    await user.click(screen.getByRole("button", { name: "Automático" }));
    expect(document.documentElement.dataset.theme).toBeUndefined();
    expect(localStorage.getItem("theme")).toBeNull();
  });

  it("signing out ends the session and returns to the login", async () => {
    const fetch = mockFetch({ status: 204 });
    const user = renderSettings();
    await user.click(screen.getByRole("button", { name: "Sair" }));
    expect(await screen.findByText("tela de login")).toBeTruthy();
    expect(fetch.mock.calls[0][0]).toBe("/api/auth/logout");
    expect(fetch.mock.calls[0][1]!.method).toBe("POST");
  });

  it("signing out works even if the call fails (offline)", async () => {
    const fetch = mockFetch();
    fetch.mockRejectedValue(new Error("offline"));
    const user = renderSettings();
    await user.click(screen.getByRole("button", { name: "Sair" }));
    await waitFor(() => expect(screen.getByText("tela de login")).toBeTruthy());
  });
});
