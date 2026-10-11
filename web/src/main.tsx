import "@fontsource-variable/bricolage-grotesque";
import "@fontsource-variable/instrument-sans";
import "./styles.css";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { ApiError } from "./api/client";
import { App } from "./App";
import { applyStoredTheme } from "./lib/theme";

applyStoredTheme();

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // 4xx errors (expired session, no permission) do not improve by retrying
      retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
      refetchOnWindowFocus: false,
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
