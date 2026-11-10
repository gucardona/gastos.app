import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useEvents } from "./queries";

class FakeEventSource {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;
  static all: FakeEventSource[] = [];
  onmessage: (() => void) | null = null;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  readyState = 0;
  closed = false;
  constructor(readonly url: string, readonly init?: EventSourceInit) {
    FakeEventSource.all.push(this);
  }
  close() {
    this.closed = true;
    this.readyState = 2;
  }
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  /** What the browser does when a reconnect gets a non-200 (a 502 during a deploy): it gives up for good. */
  fail() {
    this.readyState = 2;
    this.onerror?.();
  }
}

const latest = () => FakeEventSource.all[FakeEventSource.all.length - 1];

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

  it("a transient error leaves the reconnect to the browser", async () => {
    const { es } = setup();
    act(() => es.open());
    act(() => { es.readyState = 0; es.onerror?.(); });
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(FakeEventSource.all).toHaveLength(1);
  });

  it("a stream the browser gave up on is reopened after 5 s, and opening it refreshes everything", async () => {
    const { es, meRefreshes } = setup();
    act(() => es.open());
    act(() => es.fail());
    expect(es.closed).toBe(true);
    await act(() => vi.advanceTimersByTimeAsync(4_999));
    expect(FakeEventSource.all).toHaveLength(1);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(FakeEventSource.all).toHaveLength(2);
    expect(latest().url).toBe("/api/events");
    act(() => latest().open());
    await act(() => vi.advanceTimersByTimeAsync(200));
    expect(meRefreshes()).toBe(1);
  });

  it("the wait grows on repeated failures, up to 30 s, and resets after a successful open", async () => {
    setup();
    const waits: number[] = [];
    for (let i = 0; i < 5; i++) {
      const before = FakeEventSource.all.length;
      act(() => latest().fail());
      let waited = 0;
      while (FakeEventSource.all.length === before && waited < 60_000) {
        await act(() => vi.advanceTimersByTimeAsync(1_000));
        waited += 1_000;
      }
      waits.push(waited);
    }
    expect(waits).toEqual([5_000, 10_000, 20_000, 30_000, 30_000]);

    act(() => latest().open());
    act(() => latest().fail());
    await act(() => vi.advanceTimersByTimeAsync(5_000));
    expect(FakeEventSource.all).toHaveLength(7);
  });

  it("coming back to the page reopens a stream the browser gave up on, without waiting", async () => {
    const { es } = setup();
    act(() => es.open());
    act(() => setHidden(true));
    act(() => es.fail());
    act(() => setHidden(false));
    expect(FakeEventSource.all).toHaveLength(2);
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(FakeEventSource.all).toHaveLength(2); // the pending retry was dropped
  });

  it("unmounting cancels a pending reopen", async () => {
    const { es, unmount } = setup();
    act(() => es.fail());
    unmount();
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(FakeEventSource.all).toHaveLength(1);
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
