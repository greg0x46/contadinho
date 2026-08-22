import { PageContainer } from "@ant-design/pro-layout";

import { ProjectionSummaryCard } from "../components/home/ProjectionSummaryCard";
import { SpendingByCategoryCard } from "../components/home/SpendingByCategoryCard";
import { TotalDebtCard } from "../components/home/TotalDebtCard";
import { TotalReceivableCard } from "../components/home/TotalReceivableCard";

export function HomePage() {
  return (
    <PageContainer title="Início">
      <div className="dashboard-layout">
        <div className="dashboard-current-summary">
          <TotalDebtCard />
          <TotalReceivableCard />
          <SpendingByCategoryCard />
        </div>
        <ProjectionSummaryCard />
      </div>
    </PageContainer>
  );
}
