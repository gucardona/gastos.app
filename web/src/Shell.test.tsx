import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useApp } from "./context";
import { Shell } from "./Shell";
import { ana, categories, members, mockFetch, paymentMethods, wallet } from "./test/fixtures";

class FakeEventSource {
  static all: FakeEventSource[] = [];
  onmessage: (() => void) | null = null;
  closed = false;
  constructor(readonly url: string, readonly init?: EventSourceInit) {
    FakeEventSource.all.push(this);
  }
  close() {
    this.closed = true;
  }
}

const viagem = { ...wallet, id: 2, name: "Viagem" };

function Probe() {
  const a = useApp();
  return (
    <>
      <p>mês {a.month}</p>
      <p>carteira {a.wallet.name}</p>
      <button type="button" onClick={() => a.setWalletId(2)}>trocar de carteira</button>
      <button type="button" onClick={() => a.setMonth("2026-05")}>ir para maio</button>
      <input aria-label="campo de texto" />
    </>
  );
}

type Me = { user: typeof ana; wallets: (typeof wallet)[] };

/** Serves the whole API the Shell consumes; `me` can be swapped for an error status. */
function serve(me: Me | { status: number } = { user: ana, wallets: [wallet, viagem] }, extra: Record<string, unknown> = {}) {
  return mockFetch((url) => {
    if (url === "/api/me") return "status" in me ? { status: me.status, body: { error: { code: "x", message: "falhou" } } } : { body: me };
    for (const [suffix, body] of Object.entries({ "/categories": categories, "/payment-methods": paymentMethods, "/members": members, ...extra })) {
      if (url.endsWith(suffix)) return { body };
    }
    return { body: {} };
  });
}

function renderShell(route = "/") {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route path="/login" element={<p>tela de login</p>} />
          <Route element={<Shell />}>
            <Route index element={<Probe />} />
            <Route path="lancamentos" element={<p>página de lançamentos</p>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...view, user: userEvent.setup() };
}

const urls = (fetch: ReturnType<typeof mockFetch>) => fetch.mock.calls.map((c) => c[0] as string);

beforeEach(() => {
  FakeEventSource.all = [];
  localStorage.clear();
  vi.stubGlobal("EventSource", FakeEventSource);
});

describe("Shell: session and loading", () => {
  it("without a session, goes to the login", async () => {
    serve({ status: 401 });
    renderShell();
    expect(await screen.findByText("tela de login")).toBeTruthy();
  });

  it("shows loading and then the page inside the shell", async () => {
    serve();
    const { container } = renderShell();
    expect(container.querySelectorAll(".skeleton").length).toBeGreaterThan(0);
    expect(await screen.findByText("carteira Casa")).toBeTruthy();
    expect(container.querySelector(".skeleton")).toBeNull();
    for (const name of ["Visão", "Lançamentos", "Caixa", "Ajustes"]) {
      expect(screen.getAllByRole("link", { name: new RegExp(name) }).length).toBeGreaterThan(0);
    }
    // Recorrentes lives inside Lançamentos, and Divisão only exists for wallets that split expenses
    expect(screen.queryByRole("link", { name: /Recorrentes|Divisão/ })).toBeNull();
  });

  it("offers Divisão when the active wallet splits expenses", async () => {
    serve({ user: ana, wallets: [{ ...wallet, splittingEnabled: true }] });
    renderShell();
    await screen.findByText("carteira Casa");
    expect(screen.getAllByRole("link", { name: "Divisão" }).length).toBeGreaterThan(0);
  });

  it("the current page's link is marked as current", async () => {
    serve();
    renderShell("/lancamentos");
    await screen.findByText("página de lançamentos");
    const current = screen.getAllByRole("link").filter((l) => l.getAttribute("aria-current") === "page");
    expect(current.length).toBeGreaterThan(0);
    expect(current.every((l) => /Lançamentos/.test(l.textContent ?? ""))).toBe(true);
  });

  it("a network error on /me shows the failure and allows a retry", async () => {
    const fetch = serve({ status: 500 });
    const user = userEvent.setup();
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={qc}><MemoryRouter><Routes><Route element={<Shell />}><Route index element={<Probe />} /></Route></Routes></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText(/Não foi possível carregar seus dados/)).toBeTruthy();
    const before = urls(fetch).filter((u) => u === "/api/me").length;
    await user.click(screen.getByRole("button", { name: "Tentar de novo" }));
    await waitFor(() => expect(urls(fetch).filter((u) => u === "/api/me").length).toBeGreaterThan(before));
  });

  it("a failure loading the wallet data shows the failure too", async () => {
    mockFetch((url) => (url === "/api/me" ? { body: { user: ana, wallets: [wallet] } } : url.endsWith("/members") ? { body: members } : { status: 500, body: { error: { code: "x", message: "falhou" } } }));
    renderShell();
    expect(await screen.findByText(/Não foi possível carregar seus dados/)).toBeTruthy();
  });

  it("a user with no wallet gets a clear notice", async () => {
    serve({ user: ana, wallets: [] });
    renderShell();
    expect(await screen.findByText("Você não participa de nenhuma carteira.")).toBeTruthy();
  });
});

describe("Shell: active wallet and month", () => {
  it("uses the first wallet by default and the current month", async () => {
    serve();
    renderShell();
    expect(await screen.findByText("carteira Casa")).toBeTruthy();
    expect(screen.getByText(/^mês \d{4}-\d{2}$/)).toBeTruthy();
  });

  it("uses the wallet stored in the browser", async () => {
    localStorage.setItem("walletId", "2");
    const fetch = serve();
    renderShell();
    expect(await screen.findByText("carteira Viagem")).toBeTruthy();
    expect(urls(fetch)).toContain("/api/wallets/2/categories");
    expect(urls(fetch)).not.toContain("/api/wallets/1/categories");
  });

  it("ignores a stored wallet that no longer exists", async () => {
    localStorage.setItem("walletId", "99");
    serve();
    renderShell();
    expect(await screen.findByText("carteira Casa")).toBeTruthy();
  });

  it("switching wallet stores the choice and reloads its data", async () => {
    const fetch = serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    await user.click(screen.getByRole("button", { name: "trocar de carteira" }));
    expect(await screen.findByText("carteira Viagem")).toBeTruthy();
    expect(localStorage.getItem("walletId")).toBe("2");
    await waitFor(() => expect(urls(fetch)).toContain("/api/wallets/2/members"));
  });

  it("the month comes from the URL and changing month updates the URL", async () => {
    serve();
    const { user } = renderShell("/?m=2026-03");
    expect(await screen.findByText("mês 2026-03")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "ir para maio" }));
    expect(await screen.findByText("mês 2026-05")).toBeTruthy();
  });
});

describe("Shell: real time (SSE)", () => {
  it("opens the active wallet's stream with credentials and closes on unmount", async () => {
    serve();
    const { unmount } = renderShell();
    await screen.findByText("carteira Casa");
    await waitFor(() => expect(FakeEventSource.all).toHaveLength(1));
    expect(FakeEventSource.all[0].url).toBe("/api/wallets/1/events");
    expect(FakeEventSource.all[0].init).toEqual({ withCredentials: true });
    unmount();
    expect(FakeEventSource.all[0].closed).toBe(true);
  });

  it("opens no stream until it knows which wallet is active", () => {
    serve({ status: 401 });
    renderShell();
    expect(FakeEventSource.all).toHaveLength(0);
  });

  it("a server notice refreshes the wallet's data", async () => {
    const fetch = serve();
    renderShell();
    await screen.findByText("carteira Casa");
    await waitFor(() => expect(FakeEventSource.all).toHaveLength(1));
    const before = urls(fetch).filter((u) => u === "/api/wallets/1/categories").length;
    await act(async () => FakeEventSource.all[0].onmessage?.());
    await waitFor(() => expect(urls(fetch).filter((u) => u === "/api/wallets/1/categories").length).toBeGreaterThan(before));
  });

  it("when switching wallet, closes the old stream and opens the new one", async () => {
    serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    await waitFor(() => expect(FakeEventSource.all).toHaveLength(1));
    await user.click(screen.getByRole("button", { name: "trocar de carteira" }));
    await waitFor(() => expect(FakeEventSource.all).toHaveLength(2));
    expect(FakeEventSource.all[0].closed).toBe(true);
    expect(FakeEventSource.all[1].url).toBe("/api/wallets/2/events");
  });
});

describe("Shell: quick add", () => {
  const dialogOpen = () => document.querySelector("dialog[open]") !== null;

  it("the N key opens quick add", async () => {
    serve();
    renderShell();
    await screen.findByText("carteira Casa");
    expect(dialogOpen()).toBe(false);
    fireEvent.keyDown(document.body, { key: "n" });
    await waitFor(() => expect(dialogOpen()).toBe(true));
    expect(screen.getByRole("textbox", { name: "Valor" })).toBeTruthy();
  });

  it("the 'Novo lançamento' button opens it too", async () => {
    serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    await user.click(screen.getAllByRole("button", { name: "Novo lançamento" })[0]);
    await waitFor(() => expect(dialogOpen()).toBe(true));
  });

  it("typing the letter n in a field opens nothing", async () => {
    serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    await user.type(screen.getByLabelText("campo de texto"), "banana");
    expect(dialogOpen()).toBe(false);
  });

  it("Ctrl+N and Cmd+N are left to the browser", async () => {
    serve();
    renderShell();
    await screen.findByText("carteira Casa");
    fireEvent.keyDown(document.body, { key: "n", ctrlKey: true });
    fireEvent.keyDown(document.body, { key: "n", metaKey: true });
    expect(dialogOpen()).toBe(false);
  });

  it("readers have neither the button nor the shortcut", async () => {
    serve({ user: ana, wallets: [{ ...wallet, role: "reader" }] });
    renderShell();
    await screen.findByText("carteira Casa");
    expect(screen.queryByRole("button", { name: "Novo lançamento" })).toBeNull();
    fireEvent.keyDown(document.body, { key: "n" });
    expect(dialogOpen()).toBe(false);
  });

  it("closing quick add with the button closes the dialog", async () => {
    serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    fireEvent.keyDown(document.body, { key: "n" });
    await waitFor(() => expect(dialogOpen()).toBe(true));
    await user.click(screen.getByRole("button", { name: "Fechar" }));
    await waitFor(() => expect(dialogOpen()).toBe(false));
  });
});
