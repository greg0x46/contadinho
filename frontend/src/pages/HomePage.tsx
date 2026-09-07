import { PageContainer } from "@ant-design/pro-layout";

import { ProjectionSummaryCard } from "../components/home/ProjectionSummaryCard";
import { SpendingByCategoryCard } from "../components/home/SpendingByCategoryCard";
import { PendingBalanceCard } from "../components/home/PendingBalanceCard";

export function HomePage() {
  return (
    <PageContainer title="Início">
      <div className="dashboard-layout">
        <div className="dashboard-current-summary">
          <PendingBalanceCard />
          <SpendingByCategoryCard />
        </div>
        <ProjectionSummaryCard />
      </div>
    </PageContainer>
  );
}
