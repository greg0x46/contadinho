import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";

import { createAppQueryClient } from "./app/queryClient";
import { AppRouter } from "./app/router";
import "antd/dist/reset.css";
import "./styles/global.css";
import "./styles/sync-runs.css";
import "./styles/transactions.css";
import "./styles/home.css";

const root = document.getElementById("root");

if (root === null) {
  throw new Error("Elemento raiz da aplicação não encontrado.");
}

const queryClient = createAppQueryClient();

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AppRouter />
    </QueryClientProvider>
  </StrictMode>,
);
