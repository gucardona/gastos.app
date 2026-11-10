import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useEvents } from "./queries";

class FakeEventSource {
  static all: FakeEventSource[] = [];
  onmessage: (() => void) | null = null;
  onopen: (() => void) | null = null;
  closed = false;
  constructor(readonly url: string, readonly init?: EventSourceInit) {
    FakeEventSource.all.push(this);
  }
  close() {
    this.closed = true;
  }
}

function setup(enabled = true) {
  const qc = new QueryClient();
  const invalidate = vi.spyOn(qc, "invalidateQueries").mockResolvedValue();
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  const view = renderHook(() => useEvents(enabled), { wrapper });
  const meRefreshes = () => invalidate.mock.calls.filter(([f]) => JSON.stringify(f?.queryKey) === '["me"]').length;
  return { ...view, qc, invalidate, meRefreshes, es: FakeEventSource.all[0] };
}

const setHidden = (hidden: boolean) => {
  Object.defineProperty(document, "hidden", { configurable: true, get: () => hidden });
  document.dispatchEvent(new Event("visibilitychange"));
};

beforeEach(() => {
  FakeEventSource.all = [];
  vi.stubGlobal("EventSource", FakeEventSource);
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
});

describe("useEvents", () => {
  it("opens the user's stream once, with credentials, and closes it on unmount", () => {
    const { es, unmount } = setup();
    expect(FakeEventSource.all).toHaveLength(1);
    expect(es.url).toBe("/api/events");
    expect(es.init).toEqual({ withCredentials: true });
    unmount();
    expect(es.closed).toBe(true);
  });

  it("opens nothing while disabled (no session yet)", () => {
    setup(false);
    expect(FakeEventSource.all).toHaveLength(0);
  });

  it("a notice refreshes me and the wallets' data", async () => {
    const { es, invalidate, meRefreshes } = setup();
    act(() => es.onmessage?.());
    expect(invalidate).not.toHaveBeenCalled(); // debounced
    await act(() => vi.advanceTimersByTimeAsync(150));
    expect(meRefreshes()).toBe(1);
    expect(invalidate.mock.calls.some(([f]) => f?.queryKey?.[0] === "wallet")).toBe(true);
  });

  it("a burst of notices causes a single refresh", async () => {
    const { es, meRefreshes } = setup();
    for (let i = 0; i < 5; i++) {
      act(() => es.onmessage?.());
      await act(() => vi.advanceTimersByTimeAsync(50));
    }
    await act(() => vi.advanceTimersByTimeAsync(150));
    expect(meRefreshes()).toBe(1);
  });

  it("the first open refreshes nothing, a reconnect refreshes everything", async () => {
    const { es, meRefreshes } = setup();
    act(() => es.onopen?.());
    await act(() => vi.advanceTimersByTimeAsync(200));
    expect(meRefreshes()).toBe(0);
    act(() => es.onopen?.());
    await act(() => vi.advanceTimersByTimeAsync(200));
    expect(meRefreshes()).toBe(1);
  });

  it("coming back to the page after a long while refreshes, a quick glance away does not", async () => {
    const { meRefreshes } = setup();
    act(() => setHidden(true));
    await act(() => vi.advanceTimersByTimeAsync(5_000));
    act(() => setHidden(false));
    await act(() => vi.advanceTimersByTimeAsync(200));
    expect(meRefreshes()).toBe(0);

    act(() => setHidden(true));
    await act(() => vi.advanceTimersByTimeAsync(31_000));
    act(() => setHidden(false));
    await act(() => vi.advanceTimersByTimeAsync(200));
    expect(meRefreshes()).toBe(1);
  });

  it("nothing fires after unmount", async () => {
    const { es, unmount, invalidate } = setup();
    act(() => es.onmessage?.());
    unmount();
    act(() => setHidden(true));
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    act(() => setHidden(false));
    await act(() => vi.advanceTimersByTimeAsync(200));
    expect(invalidate).not.toHaveBeenCalled();
  });
});
