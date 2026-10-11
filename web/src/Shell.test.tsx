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
      <button onClick={() => a.setWalletId(2)}>trocar de carteira</button>
      <button onClick={() => a.setMonth("2026-05")}>ir para maio</button>
      <input aria-label="campo de texto" />
    </>
  );
}

type Me = { user: typeof ana; wallets: (typeof wallet)[] };

/** Responde a API inteira que o Shell consome; `me` pode ser trocado por um status de erro. */
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

describe("Shell: sessão e carregamento", () => {
  it("sem sessão, leva ao login", async () => {
    serve({ status: 401 });
    renderShell();
    expect(await screen.findByText("tela de login")).toBeTruthy();
  });

  it("mostra o carregamento e depois a página dentro da casca", async () => {
    serve();
    const { container } = renderShell();
    expect(container.querySelectorAll(".skeleton").length).toBeGreaterThan(0);
    expect(await screen.findByText("carteira Casa")).toBeTruthy();
    expect(container.querySelector(".skeleton")).toBeNull();
    for (const name of ["Visão geral", "Lançamentos", "Recorrentes", "Divisão", "Ajustes"]) {
      expect(screen.getAllByRole("link", { name: new RegExp(name) }).length).toBeGreaterThan(0);
    }
  });

  it("o link da página atual fica marcado como atual", async () => {
    serve();
    renderShell("/lancamentos");
    await screen.findByText("página de lançamentos");
    const current = screen.getAllByRole("link").filter((l) => l.getAttribute("aria-current") === "page");
    expect(current.length).toBeGreaterThan(0);
    expect(current.every((l) => /Lançamentos/.test(l.textContent ?? ""))).toBe(true);
  });

  it("erro de rede em /me mostra a falha e deixa tentar de novo", async () => {
    const fetch = serve({ status: 500 });
    const user = userEvent.setup();
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={qc}><MemoryRouter><Routes><Route element={<Shell />}><Route index element={<Probe />} /></Route></Routes></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText(/Não foi possível carregar seus dados/)).toBeTruthy();
    const before = urls(fetch).filter((u) => u === "/api/me").length;
    await user.click(screen.getByRole("button", { name: "Tentar de novo" }));
    await waitFor(() => expect(urls(fetch).filter((u) => u === "/api/me").length).toBeGreaterThan(before));
  });

  it("falha nos dados da carteira também mostra a falha", async () => {
    mockFetch((url) => (url === "/api/me" ? { body: { user: ana, wallets: [wallet] } } : url.endsWith("/members") ? { body: members } : { status: 500, body: { error: { code: "x", message: "falhou" } } }));
    renderShell();
    expect(await screen.findByText(/Não foi possível carregar seus dados/)).toBeTruthy();
  });

  it("usuário sem nenhuma carteira recebe um aviso claro", async () => {
    serve({ user: ana, wallets: [] });
    renderShell();
    expect(await screen.findByText("Você não participa de nenhuma carteira.")).toBeTruthy();
  });
});

describe("Shell: carteira e mês ativos", () => {
  it("usa a primeira carteira por padrão e o mês corrente", async () => {
    serve();
    renderShell();
    expect(await screen.findByText("carteira Casa")).toBeTruthy();
    expect(screen.getByText(/^mês \d{4}-\d{2}$/)).toBeTruthy();
  });

  it("usa a carteira guardada no navegador", async () => {
    localStorage.setItem("walletId", "2");
    const fetch = serve();
    renderShell();
    expect(await screen.findByText("carteira Viagem")).toBeTruthy();
    expect(urls(fetch)).toContain("/api/wallets/2/categories");
    expect(urls(fetch)).not.toContain("/api/wallets/1/categories");
  });

  it("ignora uma carteira guardada que não existe mais", async () => {
    localStorage.setItem("walletId", "99");
    serve();
    renderShell();
    expect(await screen.findByText("carteira Casa")).toBeTruthy();
  });

  it("trocar de carteira guarda a escolha e recarrega os dados dela", async () => {
    const fetch = serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    await user.click(screen.getByRole("button", { name: "trocar de carteira" }));
    expect(await screen.findByText("carteira Viagem")).toBeTruthy();
    expect(localStorage.getItem("walletId")).toBe("2");
    await waitFor(() => expect(urls(fetch)).toContain("/api/wallets/2/members"));
  });

  it("o mês vem da URL e a troca de mês atualiza a URL", async () => {
    serve();
    const { user } = renderShell("/?m=2026-03");
    expect(await screen.findByText("mês 2026-03")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "ir para maio" }));
    expect(await screen.findByText("mês 2026-05")).toBeTruthy();
  });
});

describe("Shell: tempo real (SSE)", () => {
  it("abre o fluxo da carteira ativa com credenciais e fecha ao sair", async () => {
    serve();
    const { unmount } = renderShell();
    await screen.findByText("carteira Casa");
    await waitFor(() => expect(FakeEventSource.all).toHaveLength(1));
    expect(FakeEventSource.all[0].url).toBe("/api/wallets/1/events");
    expect(FakeEventSource.all[0].init).toEqual({ withCredentials: true });
    unmount();
    expect(FakeEventSource.all[0].closed).toBe(true);
  });

  it("não abre fluxo nenhum enquanto não sabe qual é a carteira", () => {
    serve({ status: 401 });
    renderShell();
    expect(FakeEventSource.all).toHaveLength(0);
  });

  it("um aviso do servidor renova os dados da carteira", async () => {
    const fetch = serve();
    renderShell();
    await screen.findByText("carteira Casa");
    await waitFor(() => expect(FakeEventSource.all).toHaveLength(1));
    const before = urls(fetch).filter((u) => u === "/api/wallets/1/categories").length;
    await act(async () => FakeEventSource.all[0].onmessage?.());
    await waitFor(() => expect(urls(fetch).filter((u) => u === "/api/wallets/1/categories").length).toBeGreaterThan(before));
  });

  it("ao trocar de carteira, fecha o fluxo antigo e abre o da nova", async () => {
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

describe("Shell: lançamento rápido", () => {
  const dialogOpen = () => document.querySelector("dialog[open]") !== null;

  it("a tecla N abre o lançamento rápido", async () => {
    serve();
    renderShell();
    await screen.findByText("carteira Casa");
    expect(dialogOpen()).toBe(false);
    fireEvent.keyDown(document.body, { key: "n" });
    await waitFor(() => expect(dialogOpen()).toBe(true));
    expect(screen.getByRole("textbox", { name: "Valor" })).toBeTruthy();
  });

  it("o botão 'Novo lançamento' também abre", async () => {
    serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    await user.click(screen.getAllByRole("button", { name: "Novo lançamento" })[0]);
    await waitFor(() => expect(dialogOpen()).toBe(true));
  });

  it("digitar a letra n num campo não abre nada", async () => {
    serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    await user.type(screen.getByLabelText("campo de texto"), "banana");
    expect(dialogOpen()).toBe(false);
  });

  it("Ctrl+N e Cmd+N seguem para o navegador", async () => {
    serve();
    renderShell();
    await screen.findByText("carteira Casa");
    fireEvent.keyDown(document.body, { key: "n", ctrlKey: true });
    fireEvent.keyDown(document.body, { key: "n", metaKey: true });
    expect(dialogOpen()).toBe(false);
  });

  it("leitores não têm botão nem atalho", async () => {
    serve({ user: ana, wallets: [{ ...wallet, role: "reader" }] });
    renderShell();
    await screen.findByText("carteira Casa");
    expect(screen.queryByRole("button", { name: "Novo lançamento" })).toBeNull();
    fireEvent.keyDown(document.body, { key: "n" });
    expect(dialogOpen()).toBe(false);
  });

  it("fechar o lançamento rápido pelo botão fecha o dialog", async () => {
    serve();
    const { user } = renderShell();
    await screen.findByText("carteira Casa");
    fireEvent.keyDown(document.body, { key: "n" });
    await waitFor(() => expect(dialogOpen()).toBe(true));
    await user.click(screen.getByRole("button", { name: "Fechar" }));
    await waitFor(() => expect(dialogOpen()).toBe(false));
  });
});
