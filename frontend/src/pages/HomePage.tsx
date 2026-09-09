import { PageContainer } from "@ant-design/pro-layout";

import { ProjectionSummaryCard } from "../components/home/ProjectionSummaryCard";
import { SpendingByCategoryCard } from "../components/home/SpendingByCategoryCard";
import { PendingBalanceCard } from "../components/home/PendingBalanceCard";

import { PeriodNavigator } from "../components/filters/PeriodNavigator";
import { periodPresets } from "../components/filters/periodPresets";
import { useHomePeriod } from "../hooks/useHomePeriod";

export function HomePage() {
  const { period, setPeriod } = useHomePeriod();
  return (
    <PageContainer
      title="Início"
      extra={
        <div className="dashboard-period">
          <PeriodNavigator id="home-period" value={[period.from, period.to]}
            presets={periodPresets()} onChange={setPeriod}
            reset={{ preset: "this-month", label: "Este mês" }} bare />
        </div>
      }
    >
      <div className="dashboard-layout">
        <div className="dashboard-current-summary">
          <PendingBalanceCard />
          <SpendingByCategoryCard period={period} />
        </div>
        <ProjectionSummaryCard period={period} />
      </div>
    </PageContainer>
  );
}
