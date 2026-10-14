import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { appCtx, categories, mockFetch, renderApp, wallet } from "../../test/fixtures";
import { CategoriesSection } from "./CategoriesSection";

const reader = () => appCtx({ canEdit: false, wallet: { ...wallet, role: "reader" } });

describe("Categories screen", () => {
  it("lists the expense categories with their kind of expense, and switches to income", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<CategoriesSection />);
    expect(screen.getByRole("button", { name: /Mercado/ }).textContent).toContain("Essencial");
    expect(screen.queryByText("Salário")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Entradas" }));
    expect(screen.getByRole("button", { name: /Salário/ }).textContent).toContain("Entrada");
    expect(screen.queryByText("Mercado")).toBeNull();
  });

  it("tapping a category opens it for editing", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<CategoriesSection />);
    await user.click(screen.getByRole("button", { name: /Mercado/ }));
    expect((screen.getByLabelText("Nome da categoria") as HTMLInputElement).value).toBe("Mercado");
  });

  it("'Nova categoria' creates in the kind being shown", async () => {
    const fetch = mockFetch({ body: [] });
    const user = userEvent.setup();
    renderApp(<CategoriesSection />);
    await user.click(screen.getByRole("button", { name: "Entradas" }));
    await user.click(screen.getByRole("button", { name: "Nova categoria" }));
    await user.type(screen.getByLabelText("Nome da categoria"), "Bônus");
    expect(screen.queryByRole("group", { name: "Tipo de gasto" })).toBeNull(); // income has none
    await user.click(screen.getByRole("button", { name: "Criar categoria" }));
    const post = fetch.mock.calls.find((c) => c[1]?.method === "POST")!;
    expect(JSON.parse(post[1]!.body as string).kind).toBe("income");
  });

  it("archived categories are behind a toggle, and open to be restored", async () => {
    mockFetch({ body: [] });
    const user = userEvent.setup();
    const old = { ...categories[1], archived: true };
    renderApp(<CategoriesSection />, appCtx({ categories: [categories[0], old, categories[2]] }));
    expect(screen.queryByText("Restaurante")).toBeNull();
    await user.click(screen.getByRole("button", { name: /Mostrar arquivadas \(1\)/ }));
    await user.click(screen.getByRole("button", { name: /Restaurante/ }));
    expect(screen.getByRole("button", { name: "Restaurar" })).toBeTruthy();
  });

  it("a reader sees the categories but cannot create or open them", () => {
    renderApp(<CategoriesSection />, reader());
    expect(screen.getByText("Mercado")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Nova categoria" })).toBeNull();
    expect(screen.queryByRole("button", { name: /Mercado/ })).toBeNull();
  });
});
