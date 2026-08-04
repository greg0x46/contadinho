import { PageContainer } from "@ant-design/pro-layout";

import { SpendingByCategoryCard } from "../components/home/SpendingByCategoryCard";
import { TotalDebtCard } from "../components/home/TotalDebtCard";
import { TotalReceivableCard } from "../components/home/TotalReceivableCard";

export function HomePage() {
  return (
    <PageContainer title="Início">
      <div className="dashboard-grid">
        <TotalDebtCard />
        <TotalReceivableCard />
        <SpendingByCategoryCard />
      </div>
    </PageContainer>
  );
}
