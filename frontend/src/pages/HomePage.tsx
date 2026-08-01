import { PageContainer } from "@ant-design/pro-layout";

import { SpendingByCategoryCard } from "../components/home/SpendingByCategoryCard";
import { TotalDebtCard } from "../components/home/TotalDebtCard";

export function HomePage() {
  return (
    <PageContainer title="Início">
      <div className="dashboard-grid">
        <TotalDebtCard />
        <SpendingByCategoryCard />
      </div>
    </PageContainer>
  );
}
