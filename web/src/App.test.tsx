import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { ana, categories, members, mockFetch, paymentMethods, wallet } from "./test/fixtures";

describe("App routes", () => {
  it("the old /recorrentes address lands on Lançamentos", async () => {
    vi.stubGlobal("EventSource", class { onmessage = null; close() {} });
    mockFetch((url) => {
      if (url === "/api/me") return { body: { user: ana, assistantEnabled: false, wallets: [wallet] } };
      for (const [suffix, body] of Object.entries({ "/categories": categories, "/payment-methods": paymentMethods, "/members": members })) {
        if (url.endsWith(suffix)) return { body };
      }
      return { body: [] };
    });
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter initialEntries={["/recorrentes"]}><App /></MemoryRouter>
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("searchbox", { name: "Buscar" })).toBeTruthy(); // only the Lançamentos page has it
  });
});
