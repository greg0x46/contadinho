import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Card, Empty, Flex, Statistic } from "antd";
import dayjs from "dayjs";
import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import type { CategoryImpact, MonthSummary } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { AccumulatedResultCard } from "../components/timeline/AccumulatedResultCard";
import { CategoryEvolutionChart } from "../components/timeline/CategoryEvolutionChart";
import { CategoryImpactList } from "../components/timeline/CategoryImpactList";
import { SummaryCards } from "../components/timeline/SummaryCards";
import { TimeNavigator } from "../components/timeline/TimeNavigator";
import { formatBRL, sumBRL } from "../presentation/money";
import { useTimeline } from "../hooks/useTimeline";
import { colors } from "../theme/tokens";

const dateFormat = "YYYY-MM-DD";
const monthFormat = "YYYY-MM";

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
          <span style={{ fontSize: 12, color: isUp ? colors.success : colors.error }}>
            {isUp ? "↑" : "↓"} {comparison.delta_percent.replace("-", "")}%
          </span>
        }
      />
    </Card>
  );
}

// Aportes and resgates are reported apart from receitas/despesas: the money
// leaves (or returns to) the caixa but stays in the patrimônio, so counting
// them as expense or income would double-read the same movement. The backend
// already keeps income/expense free of the transferred parcel.
function InvestmentMovementCards({
  selectedMonth,
  yearSummaries,
}: {
  selectedMonth: MonthSummary | null;
  yearSummaries: MonthSummary[];
}) {
  const contributions = yearSummaries.map((m) => m.investment_contributions);
  const withdrawals = yearSummaries.map((m) => m.investment_withdrawals);
  // Both are absolute amounts, so a zero sum means no movement at all in the
  // period — nothing worth an extra card on the page.
  if (sumBRL([...contributions, ...withdrawals]) === "0.00") return null;
  return (
    <Card
      size="small"
      title="Movimentações de investimento"
      style={{ marginBottom: 16 }}
      className="timeline-investment-movements"
    >
      <Flex gap="large" wrap>
        <Statistic
          title="Aportes no mês"
          value={formatBRL(selectedMonth?.investment_contributions ?? "0.00")}
        />
        <Statistic
          title="Resgates no mês"
          value={formatBRL(selectedMonth?.investment_withdrawals ?? "0.00")}
        />
        <Statistic title="Aportes no ano" value={formatBRL(sumBRL(contributions))} />
        <Statistic title="Resgates no ano" value={formatBRL(sumBRL(withdrawals))} />
      </Flex>
      <p style={{ marginBottom: 0, marginTop: 12, fontSize: 12, color: colors.textSecondary }}>
        Aportes e resgates não entram em receitas nem em despesas — apenas movem dinheiro entre o
        caixa e os investimentos.
      </p>
    </Card>
  );
}

export function FinancialReportPage() {
  const [today] = useState(() => dayjs());
  const [searchParams, setSearchParams] = useSearchParams();

  // The browsed month lives in the URL: reloading or sharing the page must
  // not silently snap back to today.
  // dayjs' strict format parsing needs the customParseFormat plugin, which
  // this app doesn't load — so the shape is validated here, the same way
  // TransactionsPage validates its date range params.
  const monthParam = searchParams.get("mes");
  const parsedMonth =
    monthParam !== null && /^\d{4}-\d{2}$/.test(monthParam) ? dayjs(`${monthParam}-01`) : null;
  const month = parsedMonth?.isValid() ? parsedMonth : today.startOf("month");
  const setMonth = (next: dayjs.Dayjs) => {
    const params = new URLSearchParams(searchParams);
    params.set("mes", next.format(monthFormat));
    setSearchParams(params, { replace: true });
  };

  const [evolutionCategory, setEvolutionCategory] = useState<CategoryImpact | null>(null);

  const params = useMemo(() => {
    const yearStart = month.startOf("year");
    const yearEnd = month.endOf("year");
    // The balance anchor is always "today" — see internal/timeline/build.go's
    // buildPoints comment — so `from` must reach back to today even when
    // browsing a future year, or the entries between today and the visible
    // window would be missing from the running balance. `analysisMonth` is
    // what actually moves with the navigator: it scopes the category
    // breakdown and the two comparisons to the month being read.
    const from = today.isBefore(yearStart) ? today : yearStart;
    return {
      referenceDate: today.format(dateFormat),
      analysisMonth: month.startOf("month").format(dateFormat),
      from: from.format(dateFormat),
      to: yearEnd.format(dateFormat),
      yearOverYear: true,
      categoryEvolutionId: evolutionCategory ? (evolutionCategory.category_id ?? "none") : null,
    };
  }, [month, today, evolutionCategory]);

  const timeline = useTimeline(params);
  const selectedMonthKey = month.startOf("month").format(dateFormat);
  const selectedMonthSummary =
    timeline.monthlyBreakdown.find((m) => m.month === selectedMonthKey) ?? null;
  const monthsSoFar = timeline.monthlyBreakdown.filter((m) => m.month <= selectedMonthKey);
  const hasAnyMovement = timeline.monthlyBreakdown.length > 0;

  return (
    <PageContainer
      title="Relatório financeiro"
      subTitle="Como foi o mês"
      content="Quanto entrou, quanto saiu e onde você mais gastou no mês selecionado — e como isso se acumula no ano. Para o que ainda vai acontecer, acompanhe a projeção na Home."
    >
      <Flex align="center" gap="middle" wrap style={{ marginBottom: 16 }}>
        <TimeNavigator month={month} onChange={setMonth} />
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
          <SummaryCards selectedMonth={selectedMonthSummary} yearSummaries={monthsSoFar} />
          <InvestmentMovementCards selectedMonth={selectedMonthSummary} yearSummaries={monthsSoFar} />
          {(timeline.monthOverMonth || timeline.yearOverYear) && (
            <Flex gap="middle" wrap style={{ marginBottom: 16 }}>
              <ComparisonStatistic title="Resultado vs. mês anterior" comparison={timeline.monthOverMonth} />
              <ComparisonStatistic title="Acumulado vs. mesmo período ano anterior" comparison={timeline.yearOverYear} />
            </Flex>
          )}
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
        </>
      )}
    </PageContainer>
  );
}
