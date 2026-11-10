import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Link, MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ErrorBoundary } from "./ErrorBoundary";

let broken = true;
function Fragile() {
  if (broken) throw new Error("boom");
  return <p>Tela ok</p>;
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Link to="/outra">ir</Link>
      <ErrorBoundary>
        <Routes>
          <Route path="/" element={<p>Início</p>} />
          <Route path="/quebra" element={<Fragile />} />
          <Route path="/outra" element={<p>Outra tela</p>} />
        </Routes>
      </ErrorBoundary>
    </MemoryRouter>,
  );
  return userEvent.setup();
}

describe("ErrorBoundary", () => {
  beforeEach(() => {
    broken = true;
    vi.spyOn(console, "error").mockImplementation(() => {}); // React logs the caught error
  });
  afterEach(() => vi.restoreAllMocks());

  it("a screen that throws shows a message with a way out instead of a blank app", () => {
    renderAt("/quebra");
    expect(screen.getByText(/Algo deu errado/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Voltar ao início" }).getAttribute("href")).toBe("/");
  });

  it("'Tentar de novo' renders the screen again", async () => {
    const user = renderAt("/quebra");
    broken = false;
    await user.click(screen.getByRole("button", { name: "Tentar de novo" }));
    expect(screen.getByText("Tela ok")).toBeTruthy();
  });

  it("going to another screen clears the error", async () => {
    const user = renderAt("/quebra");
    await user.click(screen.getByRole("link", { name: "ir" }));
    expect(screen.getByText("Outra tela")).toBeTruthy();
    expect(screen.queryByText(/Algo deu errado/)).toBeNull();
  });
});
