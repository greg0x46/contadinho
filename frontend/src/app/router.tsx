import { lazy, Suspense } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";

import { HomePage } from "../pages/HomePage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { App } from "./App";
import { SetupGate } from "./SetupGate";

const SyncRunListPage = lazy(() =>
  import("../pages/SyncRunListPage").then((module) => ({
    default: module.SyncRunListPage,
  })),
);
const SyncRunDetailPage = lazy(() =>
  import("../pages/SyncRunDetailPage").then((module) => ({
    default: module.SyncRunDetailPage,
  })),
);
const TransactionsPage = lazy(() =>
  import("../pages/TransactionsPage").then((module) => ({
    default: module.TransactionsPage,
  })),
);
const AutomationRulesPage = lazy(() =>
  import("../pages/AutomationRulesPage").then((module) => ({
    default: module.AutomationRulesPage,
  })),
);
const DebtsPage = lazy(() =>
  import("../pages/DebtsPage").then((module) => ({
    default: module.DebtsPage,
  })),
);
const DebtDetailPage = lazy(() =>
  import("../pages/DebtDetailPage").then((module) => ({
    default: module.DebtDetailPage,
  })),
);
const ReceivablesPage = lazy(() =>
  import("../pages/ReceivablesPage").then((module) => ({
    default: module.ReceivablesPage,
  })),
);
const ReceivableDetailPage = lazy(() =>
  import("../pages/ReceivableDetailPage").then((module) => ({
    default: module.ReceivableDetailPage,
  })),
);
const CategoriesPage = lazy(() =>
  import("../pages/CategoriesPage").then((module) => ({
    default: module.CategoriesPage,
  })),
);

export function AppRouter() {
  return (
    <BrowserRouter>
      <Suspense fallback={<p role="status">Carregando página…</p>}>
        <Routes>
          <Route
            element={
              <SetupGate>
                <App />
              </SetupGate>
            }
          >
            <Route index element={<HomePage />} />
            <Route path="open-banking" element={<SyncRunListPage />} />
            <Route path="open-banking/sync-runs/:id" element={<SyncRunDetailPage />} />
            <Route path="transacoes" element={<TransactionsPage />} />
            <Route path="automacoes" element={<AutomationRulesPage />} />
            <Route path="dividas" element={<DebtsPage />} />
            <Route path="dividas/:id" element={<DebtDetailPage />} />
            <Route path="contas-a-receber" element={<ReceivablesPage />} />
            <Route path="contas-a-receber/:id" element={<ReceivableDetailPage />} />
            <Route path="categorias" element={<CategoriesPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
