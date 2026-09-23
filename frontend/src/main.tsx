import { ConfigProvider } from "antd";
import ptBR from "antd/locale/pt_BR";
import dayjs from "dayjs";
import "dayjs/locale/pt-br";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";

import { createAppQueryClient } from "./app/queryClient";
import { AppRouter } from "./app/router";
import { theme } from "./theme/tokens";
import "antd/dist/reset.css";
import "./styles/global.css";
import "./styles/sync-runs.css";
import "./styles/period.css";
import "./styles/layout.css";
import "./styles/transactions.css";
import "./styles/filters.css";
import "./styles/panel.css";
import "./styles/categories.css";
import "./styles/home.css";
import "./styles/payables.css";
import "./styles/timeline.css";
import "./styles/accounts.css";

dayjs.locale("pt-br");

const root = document.getElementById("root");

if (root === null) {
  throw new Error("Elemento raiz da aplicação não encontrado.");
}

const queryClient = createAppQueryClient();

createRoot(root).render(
  <StrictMode>
    <ConfigProvider locale={ptBR} theme={theme}>
      <QueryClientProvider client={queryClient}>
        <AppRouter />
      </QueryClientProvider>
    </ConfigProvider>
  </StrictMode>,
);
