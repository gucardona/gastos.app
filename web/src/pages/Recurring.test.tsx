import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Recurrence } from "../api/types";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../test/fixtures";
import { Recurring } from "./Recurring";

const netflix: Recurrence = {
  id: 5, kind: "expense", amountCents: 5590, description: "Netflix", categoryId: 1, paymentMethodId: 2, paidByUserId: 1,
  toUserId: null, dayOfMonth: 5, startDate: "2026-01-05", endDate: null, enabled: true, nextDate: "2026-11-05", splits: [],
};

function serve(list: Recurrence[]) {
  return mockFetch((_url, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: list }));
}
const writes = (fetch: ReturnType<typeof mockFetch>) => fetch.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");

describe("Recurrentes", () => {
  it("lista com dia, próxima data e valor", async () => {
    serve([netflix]);
    renderApp(<Recurring />);
    expect(await screen.findByText("Netflix")).toBeTruthy();
    expect(screen.getByText(/Todo dia 5, próxima em 5 de novembro/)).toBeTruthy();
    expect(screen.getByText(/−R\$\s55,90/)).toBeTruthy();
  });

  it("mostra o estado vazio com a dica de como criar", async () => {
    serve([]);
    renderApp(<Recurring />);
    expect(await screen.findByText(/ative "Repetir todo mês"/)).toBeTruthy();
  });

  it("pausa enviando o objeto inteiro com enabled=false", async () => {
    const fetch = serve([netflix]);
    const user = userEvent.setup();
    renderApp(<Recurring />);
    await user.click(await screen.findByRole("switch", { name: "Pausar Netflix" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    const [url, init] = writes(fetch)[0];
    expect(url).toBe("/api/wallets/1/recurrences/5");
    expect(init!.method).toBe("PUT");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toMatchObject({
      kind: "expense", amountCents: 5590, categoryId: 1, paymentMethodId: 2, dayOfMonth: 5, startDate: "2026-01-05", enabled: false,
    });
  });

  it("edita valor e dia, validando antes de enviar", async () => {
    const fetch = serve([netflix]);
    const user = userEvent.setup();
    renderApp(<Recurring />);
    await user.click(await screen.findByRole("button", { name: "Editar" }));
    await user.clear(screen.getByLabelText("Dia do mês"));
    await user.type(screen.getByLabelText("Dia do mês"), "40");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("O dia deve estar entre 1 e 31.")).toBeTruthy();
    expect(writes(fetch)).toHaveLength(0);

    await user.clear(screen.getByLabelText("Dia do mês"));
    await user.type(screen.getByLabelText("Dia do mês"), "10");
    await user.clear(screen.getByLabelText("Valor"));
    await user.type(screen.getByLabelText("Valor"), "59,90");
    await user.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toMatchObject({ amountCents: 5990, dayOfMonth: 10 });
  });

  it("exclui só depois de confirmar", async () => {
    const fetch = serve([netflix]);
    const user = userEvent.setup();
    renderApp(<Recurring />);
    await user.click(await screen.findByRole("button", { name: "Excluir" }));
    expect(writes(fetch)).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(writes(fetch)[0][1]!.method).toBe("DELETE");
  });

  it("só envia as partes da divisão quando a carteira divide gastos", async () => {
    const split = { ...netflix, splits: [{ userId: 1, shareBp: 6000, amountCents: 3354 }, { userId: 2, shareBp: 4000, amountCents: 2236 }] };
    const fetch = serve([split]);
    const user = userEvent.setup();
    renderApp(<Recurring />, appCtx({ wallet: { ...wallet, splittingEnabled: true } }));
    await user.click(await screen.findByRole("switch", { name: "Pausar Netflix" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0])).splits).toEqual([{ userId: 1, shareBp: 6000 }, { userId: 2, shareBp: 4000 }]);
  });

  it("leitores veem a lista sem os controles", async () => {
    serve([netflix]);
    renderApp(<Recurring />, appCtx({ canEdit: false }));
    await screen.findByText("Netflix");
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.queryByRole("button", { name: "Editar" })).toBeNull();
  });
});
