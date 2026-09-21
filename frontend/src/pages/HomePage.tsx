import { ProjectionSummaryCard } from "../components/home/ProjectionSummaryCard";
import { SpendingByCategoryCard } from "../components/home/SpendingByCategoryCard";
import { PeriodBalanceCard } from "../components/home/PeriodBalanceCard";

import { PeriodNavigator } from "../components/filters/PeriodNavigator";
import { periodPresets } from "../components/filters/periodPresets";
import { Page } from "../components/layout";
import { usePeriod } from "../hooks/usePeriod";

export function HomePage() {
  const { period, setPeriod } = usePeriod();
  return (
    <Page
      title="Início"
      context={
        <PeriodNavigator
          id="home-period"
          value={[period.from, period.to]}
          presets={periodPresets()}
          onChange={setPeriod}
          reset={{ preset: "this-month", label: "Este mês" }}
          bare
        />
      }
    >
      <div className="dashboard-layout">
        <div className="dashboard-current-summary">
          <PeriodBalanceCard period={period} />
          <SpendingByCategoryCard period={period} />
        </div>
        <ProjectionSummaryCard period={period} />
      </div>
    </Page>
  );
}
