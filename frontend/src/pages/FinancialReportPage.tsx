import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Card, Collapse, Empty, Flex, Statistic } from "antd";
import dayjs from "dayjs";
import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import type { CategoryImpact } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { AccumulatedResultCard } from "../components/timeline/AccumulatedResultCard";
import { BaseVsSimulationCompare } from "../components/timeline/BaseVsSimulationCompare";
import { CategoryEvolutionChart } from "../components/timeline/CategoryEvolutionChart";
import { CategoryImpactList } from "../components/timeline/CategoryImpactList";
import { MonthlyEvolutionChart } from "../components/timeline/MonthlyEvolutionChart";
import { ProjectionComposition } from "../components/timeline/ProjectionComposition";
import { ProjectionTimeline } from "../components/timeline/ProjectionTimeline";
import { ScenarioMultiSelect } from "../components/timeline/ScenarioMultiSelect";
import { SummaryCards } from "../components/timeline/SummaryCards";
import { TimeNavigator } from "../components/timeline/TimeNavigator";
import { formatBRL } from "../presentation/money";
import { useTimeline } from "../hooks/useTimeline";

const dateFormat = "YYYY-MM-DD";

function ComparisonStatistic({
  title,
  comparison,
}: {
  title: string;
  comparison: { current: string; delta_percent: string } | null;
}) {
  if (!comparison) return null;
  const isUp = !comparison.delta_percent.startsWith("-");
  return (
    <Card size="small">
      <Statistic
        title={title}
        value={formatBRL(comparison.current)}
        suffix={
          <span style={{ fontSize: 12, color: isUp ? "#008300" : "#e34948" }}>
            {isUp ? "↑" : "↓"} {comparison.delta_percent.replace("-", "")}%
          </span>
        }
      />
    </Card>
  );
}

export function FinancialReportPage() {
  const [month, setMonth] = useState(() => dayjs());
  const [today] = useState(() => dayjs());
  const [searchParams, setSearchParams] = useSearchParams();
  const scenarioIds = useMemo(
    () => (searchParams.get("scenario_ids") ?? "").split(",").filter((id) => id !== ""),
    [searchParams],
  );
  const setScenarioIds = (ids: string[]) => {
    const next = new URLSearchParams(searchParams);
    if (ids.length > 0) next.set("scenario_ids", ids.join(","));
    else next.delete("scenario_ids");
    setSearchParams(next, { replace: true });
  };

  const [evolutionCategory, setEvolutionCategory] = useState<CategoryImpact | null>(null);

  const params = useMemo(() => {
    const yearStart = month.startOf("year");
    const yearEnd = month.endOf("year");
    // The balance anchor is always "today" — see internal/timeline/build.go's
    // buildPoints comment — so `from` must reach back to today even when
    // browsing a future year, or the entries between today and the visible
    // window would be missing from the running balance.
    const from = today.isBefore(yearStart) ? today : yearStart;
    return {
      referenceDate: today.format(dateFormat),
      from: from.format(dateFormat),
      to: yearEnd.format(dateFormat),
      scenarioIds,
      yearOverYear: true,
      categoryEvolutionId: evolutionCategory ? (evolutionCategory.category_id ?? "none") : null,
    };
  }, [month, today, scenarioIds, evolutionCategory]);

  const timeline = useTimeline(params);
  const selectedMonthKey = month.startOf("month").format(dateFormat);
  const selectedMonthSummary =
    timeline.monthlyBreakdown.find((m) => m.month === selectedMonthKey) ?? null;
  const monthsSoFar = timeline.monthlyBreakdown.filter((m) => m.month <= selectedMonthKey);
  const hasAnyMovement = timeline.monthlyBreakdown.length > 0;
  const projectedPoint =
    timeline.base?.points.find((p) => p.date === month.endOf("month").format(dateFormat)) ??
    (timeline.base ? timeline.base.points[timeline.base.points.length - 1] : null);

  return (
    <PageContainer
      title="Relatório financeiro"
      subTitle="Histórico e situação atual"
      content="Acompanhe quanto entrou, quanto saiu e onde você mais gastou, mês a mês."
    >
      <Flex align="center" gap="middle" wrap style={{ marginBottom: 16 }}>
        <TimeNavigator month={month} onChange={setMonth} />
        <ScenarioMultiSelect activeIds={scenarioIds} onChange={setScenarioIds} />
      </Flex>

      {timeline.isLoading && <LoadingState>Carregando relatório…</LoadingState>}
      {timeline.error && !timeline.isLoading && (
        <UnavailableState onRetry={() => timeline.refetch()}>
          Não foi possível carregar o relatório financeiro
        </UnavailableState>
      )}

      {!timeline.isLoading && !timeline.error && !hasAnyMovement && (
        <Alert type="info" message="Sem movimentações no período." showIcon />
      )}

      {!timeline.isLoading && !timeline.error && hasAnyMovement && (
        <>
          {timeline.base && (
            <Flex gap="middle" wrap style={{ marginBottom: 16 }}>
              <Card size="small">
                <Statistic title="Saldo atual" value={formatBRL(timeline.base.starting_balance)} />
              </Card>
              {projectedPoint && (
                <Card size="small">
                  <Statistic title="Saldo projetado" value={formatBRL(projectedPoint.balance)} />
                </Card>
              )}
              <Card size="small">
                <Statistic
                  title={`Menor saldo até lá (${timeline.base.lowest_balance.date.split("-").reverse().join("/")})`}
                  value={formatBRL(timeline.base.lowest_balance.balance)}
                />
              </Card>
            </Flex>
          )}
          {timeline.base && (
            <ProjectionTimeline
              series={timeline.base}
              simulation={timeline.simulation}
              referenceDate={params.referenceDate}
            />
          )}
          {timeline.base && timeline.simulation && (
            <BaseVsSimulationCompare base={timeline.base} simulation={timeline.simulation} />
          )}
          {(timeline.monthOverMonth || timeline.yearOverYear) && (
            <Flex gap="middle" wrap style={{ marginBottom: 16 }}>
              <ComparisonStatistic title="Resultado vs. mês anterior" comparison={timeline.monthOverMonth} />
              <ComparisonStatistic title="Acumulado vs. mesmo período ano anterior" comparison={timeline.yearOverYear} />
            </Flex>
          )}
          <SummaryCards selectedMonth={selectedMonthSummary} yearSummaries={monthsSoFar} />
          <MonthlyEvolutionChart summaries={timeline.monthlyBreakdown} />
          <AccumulatedResultCard summaries={monthsSoFar} />
          {timeline.categoryBreakdown.length === 0 ? (
            <Empty description="Sem despesas no mês selecionado." />
          ) : (
            <CategoryImpactList
              impacts={timeline.categoryBreakdown}
              dateFrom={month.startOf("month").format(dateFormat)}
              dateTo={month.endOf("month").format(dateFormat)}
              onEvolution={setEvolutionCategory}
            />
          )}
          {evolutionCategory && timeline.categoryEvolution && (
            <CategoryEvolutionChart
              categoryName={evolutionCategory.category_name}
              months={timeline.categoryEvolution}
              onClose={() => setEvolutionCategory(null)}
            />
          )}
          {timeline.scenarioImpacts.length > 0 && (
            <Collapse
              className="timeline-chart"
              items={[
                {
                  key: "composition",
                  label: "Detalhamento da projeção",
                  children: <ProjectionComposition impacts={timeline.scenarioImpacts} />,
                },
              ]}
            />
          )}
        </>
      )}
    </PageContainer>
  );
}
