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
const AccountsPage = lazy(() =>
  import("../pages/AccountsPage").then((module) => ({
    default: module.AccountsPage,
  })),
);
const AccountDetailPage = lazy(() =>
  import("../pages/AccountDetailPage").then((module) => ({
    default: module.AccountDetailPage,
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
const PayablesPage = lazy(() =>
  import("../pages/PayablesPage").then((module) => ({
    default: module.PayablesPage,
  })),
);
const PayableDetailPage = lazy(() =>
  import("../pages/PayableDetailPage").then((module) => ({
    default: module.PayableDetailPage,
  })),
);
const CategoriesPage = lazy(() =>
  import("../pages/CategoriesPage").then((module) => ({
    default: module.CategoriesPage,
  })),
);
const InvestmentsPage = lazy(() =>
  import("../pages/InvestmentsPage").then((module) => ({
    default: module.InvestmentsPage,
  })),
);
const InvestmentDetailPage = lazy(() =>
  import("../pages/InvestmentDetailPage").then((module) => ({
    default: module.InvestmentDetailPage,
  })),
);
const SettingsPage = lazy(() =>
  import("../pages/SettingsPage").then((module) => ({
    default: module.SettingsPage,
  })),
);
const RecurringCommitmentsPage = lazy(() =>
  import("../pages/RecurringCommitmentsPage").then((module) => ({
    default: module.RecurringCommitmentsPage,
  })),
);
const ScenariosPage = lazy(() =>
  import("../pages/ScenariosPage").then((module) => ({
    default: module.ScenariosPage,
  })),
);
const FinancialReportPage = lazy(() =>
  import("../pages/FinancialReportPage").then((module) => ({
    default: module.FinancialReportPage,
  })),
);
const NetWorthPage = lazy(() =>
  import("../pages/NetWorthPage").then((module) => ({
    default: module.NetWorthPage,
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
            <Route path="contas-e-cartoes" element={<AccountsPage />} />
            <Route path="contas-e-cartoes/:id" element={<AccountDetailPage />} />
            <Route path="transacoes" element={<TransactionsPage />} />
            <Route path="automacoes" element={<AutomationRulesPage />} />
            <Route path="pendencias" element={<PayablesPage />} />
            <Route path="pendencias/:id" element={<PayableDetailPage />} />
            <Route path="recorrencias" element={<RecurringCommitmentsPage />} />
            <Route path="cenarios" element={<ScenariosPage />} />
            <Route path="relatorio-financeiro" element={<FinancialReportPage />} />
            <Route path="patrimonio-liquido" element={<NetWorthPage />} />
            <Route path="investimentos" element={<InvestmentsPage />} />
            <Route path="investimentos/:id" element={<InvestmentDetailPage />} />
            <Route path="categorias" element={<CategoriesPage />} />
            <Route path="configuracoes" element={<SettingsPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
