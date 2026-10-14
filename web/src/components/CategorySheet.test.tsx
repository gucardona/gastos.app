import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Category, Kind } from "../api/types";
import { bodyOf, categories, mockFetch, renderApp } from "../test/fixtures";
import { CategorySheet } from "./CategorySheet";

const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");
const bodyAt = (f: ReturnType<typeof mockFetch>, n = 0) => bodyOf(f, f.mock.calls.indexOf(writes(f)[n]));
const serve = (body: unknown = {}, status = 200) => mockFetch((_u, init) => (init.method && init.method !== "GET" ? { status, body } : { body: [] }));

function open(category: Category | "new" | null, kind: Kind = "expense") {
  const onClose = vi.fn();
  renderApp(<CategorySheet category={category} kind={kind} onClose={onClose} />);
  return { onClose, user: userEvent.setup() };
}

describe("CategorySheet: new", () => {
  it("is closed without a category", () => {
    open(null);
    expect(screen.queryByLabelText("Nome da categoria")).toBeNull();
  });

  it("creates an expense category with a name, an emoji, a color and a kind of expense", async () => {
    const fetch = serve({}, 201);
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Nome da categoria"), "Pets");
    await user.click(screen.getByRole("button", { name: "Outros" }));
    await user.click(screen.getByRole("button", { name: "🐶" }));
    await user.click(screen.getByRole("button", { name: "Cor #D4568B" }));
    await user.click(screen.getByRole("button", { name: "Supérfluo" }));
    await user.click(screen.getByRole("button", { name: "Criar categoria" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/categories");
    expect(writes(fetch)[0][1]!.method).toBe("POST");
    expect(bodyAt(fetch)).toEqual({ kind: "expense", name: "Pets", icon: "🐶", color: "#D4568B", essentiality: "nonessential" });
  });

  it("an income category has no kind of expense", async () => {
    const fetch = serve({}, 201);
    const { user } = open("new", "income");
    expect(screen.queryByRole("group", { name: "Tipo de gasto" })).toBeNull();
    await user.type(screen.getByLabelText("Nome da categoria"), "Bônus");
    await user.click(screen.getByRole("button", { name: "Criar categoria" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).not.toHaveProperty("essentiality");
    expect(bodyAt(fetch).kind).toBe("income");
  });

  it("shows the error of the API, such as a duplicate name, and stays open", async () => {
    mockFetch((_u, init) => (init.method === "POST" ? { status: 409, body: { error: { code: "conflict", message: "Já existe uma categoria com esse nome" } } } : { body: [] }));
    const { user, onClose } = open("new");
    await user.type(screen.getByLabelText("Nome da categoria"), "Mercado");
    await user.click(screen.getByRole("button", { name: "Criar categoria" }));
    expect(await screen.findByText("Já existe uma categoria com esse nome")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("previews the icon and the name as they change", async () => {
    const { user } = open("new");
    await user.type(screen.getByLabelText("Nome da categoria"), "Pets");
    expect(screen.getByTestId("category-preview").textContent).toContain("Pets");
  });
});

describe("CategorySheet: existing", () => {
  const mercado = categories[0]; // expense, essential, icon 🛒

  it("is prefilled and saves the whole object by PUT", async () => {
    const fetch = serve();
    const { user, onClose } = open(mercado);
    expect((screen.getByLabelText("Nome da categoria") as HTMLInputElement).value).toBe("Mercado");
    expect(screen.getByRole("button", { name: "Essencial" }).getAttribute("aria-pressed")).toBe("true");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/categories/1");
    expect(writes(fetch)[0][1]!.method).toBe("PUT");
    expect(bodyAt(fetch)).toEqual({ kind: "expense", name: "Mercado", icon: "🛒", color: "#112233", essentiality: "essential", sortOrder: 1, archived: false });
  });

  it("an icon typed freely in the old form is kept when saving", async () => {
    const fetch = serve();
    const { user } = open({ ...mercado, icon: "🦄", color: "#1f7d8f" });
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toMatchObject({ icon: "🦄", color: "#1f7d8f" });
  });

  it("deletes only after confirming, and closes when it was really deleted", async () => {
    const fetch = serve(undefined, 204);
    const { user, onClose } = open(mercado);
    await user.click(screen.getByRole("button", { name: "Excluir categoria" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
  });

  it("explains when the deletion became an archive, and offers to close", async () => {
    serve({ ...mercado, archived: true });
    const { user, onClose } = open(mercado);
    await user.click(screen.getByRole("button", { name: "Excluir categoria" }));
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    expect(await screen.findByText("Arquivada, porque já tem lançamentos.")).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Fechar" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("archives with a PUT that flips archived", async () => {
    const fetch = serve();
    const { user } = open(mercado);
    await user.click(screen.getByRole("button", { name: "Arquivar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyAt(fetch)).toMatchObject({ name: "Mercado", archived: true });
  });

  it("an archived category offers to restore, not to archive or delete", async () => {
    const fetch = serve();
    const { user } = open({ ...categories[1], archived: true });
    expect(screen.queryByRole("button", { name: "Arquivar" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Excluir categoria" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Restaurar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][0]).toBe("/api/wallets/1/categories/2");
    expect(bodyAt(fetch)).toMatchObject({ archived: false, name: "Restaurante" });
  });
});
