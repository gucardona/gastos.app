import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Recurrence } from "../api/types";
import { appCtx, bodyOf, mockFetch, renderApp, wallet } from "../test/fixtures";
import { RecurringSection } from "./RecurringSection";

const netflix: Recurrence = {
  id: 5, kind: "expense", amountCents: 5590, description: "Netflix", categoryId: 1, paymentMethodId: 2, paidByUserId: 1,
  toUserId: null, dayOfMonth: 5, startDate: "2026-01-05", endDate: null, enabled: true, nextDate: "2026-11-05", splits: [],
};
const settlement: Recurrence = { ...netflix, id: 6, kind: "settlement", categoryId: null, paymentMethodId: null, paidByUserId: 1, toUserId: 2, description: "", amountCents: 12000 };

function serve(list: Recurrence[]) {
  return mockFetch((_url, init) => (init.method && init.method !== "GET" ? { body: {} } : { body: list }));
}
const writes = (f: ReturnType<typeof mockFetch>) => f.mock.calls.filter((c) => c[1]?.method && c[1].method !== "GET");

describe("RecurringSection", () => {
  it("lists with the day, the next date and the amount", async () => {
    serve([netflix]);
    renderApp(<RecurringSection />);
    expect(await screen.findByRole("heading", { name: "Repetem todo mês" })).toBeTruthy();
    expect(await screen.findByText("Netflix")).toBeTruthy();
    expect(screen.getByText(/Todo dia 5, próxima em 5 de novembro/)).toBeTruthy();
    expect(screen.getByText(/−R\$\s55,90/)).toBeTruthy();
  });

  it("with none, tells an editor how to create one; a reader sees nothing", async () => {
    serve([]);
    const { unmount } = renderApp(<RecurringSection />);
    expect(await screen.findByText(/ative "Repetir todo mês"/)).toBeTruthy();
    unmount();
    const { container } = renderApp(<RecurringSection />, appCtx({ canEdit: false }));
    await waitFor(() => expect(container.textContent).toBe(""));
  });

  it("pauses by sending the whole object with enabled=false, and resumes with true", async () => {
    const fetch = serve([netflix]);
    const user = userEvent.setup();
    renderApp(<RecurringSection />);
    await user.click(await screen.findByRole("switch", { name: "Pausar Netflix" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    const [url, init] = writes(fetch)[0];
    expect(url).toBe("/api/wallets/1/recurrences/5");
    expect(init!.method).toBe("PUT");
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0]))).toMatchObject({
      kind: "expense", amountCents: 5590, categoryId: 1, paymentMethodId: 2, dayOfMonth: 5, startDate: "2026-01-05", enabled: false,
    });
  });

  it("a paused recurrence says so and offers to resume", async () => {
    const fetch = serve([{ ...netflix, enabled: false }]);
    const user = userEvent.setup();
    renderApp(<RecurringSection />);
    expect(await screen.findByText(/pausada/)).toBeTruthy();
    await user.click(screen.getByRole("switch", { name: "Retomar Netflix" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0])).enabled).toBe(true);
  });

  it("only sends the split parts when the wallet splits expenses", async () => {
    const split = { ...netflix, splits: [{ userId: 1, shareBp: 6000, amountCents: 3354 }, { userId: 2, shareBp: 4000, amountCents: 2236 }] };
    const fetch = serve([split]);
    const user = userEvent.setup();
    renderApp(<RecurringSection />, appCtx({ wallet: { ...wallet, splittingEnabled: true } }));
    await user.click(await screen.findByRole("switch", { name: "Pausar Netflix" }));
    await waitFor(() => expect(writes(fetch)).toHaveLength(1));
    expect(bodyOf(fetch, fetch.mock.calls.indexOf(writes(fetch)[0])).splits).toEqual([{ userId: 1, shareBp: 6000 }, { userId: 2, shareBp: 4000 }]);
  });

  it("tapping a row opens the edit sheet of that recurrence", async () => {
    serve([netflix]);
    const user = userEvent.setup();
    renderApp(<RecurringSection />);
    await user.click(await screen.findByRole("button", { name: /Netflix/ }));
    expect((screen.getByLabelText("Valor") as HTMLInputElement).value).toBe("55,90");
  });

  it("a settlement is named after the two people", async () => {
    serve([settlement]);
    renderApp(<RecurringSection />);
    expect(await screen.findByText("Acerto de Ana para Bia")).toBeTruthy();
  });

  it("readers see the list without a switch and without opening anything", async () => {
    serve([netflix]);
    renderApp(<RecurringSection />, appCtx({ canEdit: false }));
    await screen.findByText("Netflix");
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.queryByRole("button", { name: /Netflix/ })).toBeNull();
  });

  it("a load failure is said without breaking the page", async () => {
    mockFetch({ status: 500, body: { error: { code: "x", message: "falhou" } } });
    renderApp(<RecurringSection />);
    expect(await screen.findByText("Não foi possível carregar as recorrências.")).toBeTruthy();
  });
});

describe("RecurringSection: status line", () => {
  it("says until when a recurrence runs, when it has an end date", async () => {
    serve([{ ...netflix, endDate: "2027-01-05" }]);
    renderApp(<RecurringSection />);
    expect(await screen.findByText(/Todo dia 5, próxima em 5 de novembro, até 5 de janeiro/)).toBeTruthy();
  });

  it("a paused row is told by its text and its switch, not dimmed (dimming breaks the contrast)", async () => {
    serve([{ ...netflix, enabled: false }]);
    const { container } = renderApp(<RecurringSection />);
    await screen.findByText(/pausada/);
    expect(container.querySelector(".rec-row")?.getAttribute("style") ?? "").not.toContain("opacity");
  });
});
