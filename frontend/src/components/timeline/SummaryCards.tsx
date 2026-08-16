import { Card, Flex, Statistic } from "antd";

import type { MonthSummary } from "../../api/contracts";
import { formatBRL, sumBRL } from "../../presentation/money";

function sumSummaries(summaries: MonthSummary[]): { income: string; expense: string; result: string } {
  return {
    income: sumBRL(summaries.map((s) => s.income)),
    expense: sumBRL(summaries.map((s) => s.expense)),
    result: sumBRL(summaries.map((s) => s.result)),
  };
}

// SummaryCards keeps the selected month and the year-to-date accumulation
// as two visually distinct groups — the umbrella spec calls out never
// mixing "mês selecionado" with "acumulado no ano" in the same number.
export function SummaryCards({
  selectedMonth,
  yearSummaries,
}: {
  selectedMonth: MonthSummary | null;
  yearSummaries: MonthSummary[];
}) {
  const month = selectedMonth ?? { income: "0.00", expense: "0.00", result: "0.00" };
  const accumulated = sumSummaries(yearSummaries);

  return (
    <Flex vertical gap="middle" className="timeline-summary-cards">
      <div>
        <h2 className="timeline-summary-group-title">Mês selecionado</h2>
        <Flex gap="middle" wrap>
          <Card size="small">
            <Statistic title="Receitas" value={formatBRL(month.income)} />
          </Card>
          <Card size="small">
            <Statistic title="Despesas" value={formatBRL(month.expense)} />
          </Card>
          <Card size="small">
            <Statistic title="Resultado" value={formatBRL(month.result)} />
          </Card>
        </Flex>
      </div>
      <div>
        <h2 className="timeline-summary-group-title">Acumulado no ano</h2>
        <Flex gap="middle" wrap>
          <Card size="small">
            <Statistic title="Receitas" value={formatBRL(accumulated.income)} />
          </Card>
          <Card size="small">
            <Statistic title="Despesas" value={formatBRL(accumulated.expense)} />
          </Card>
          <Card size="small">
            <Statistic title="Resultado" value={formatBRL(accumulated.result)} />
          </Card>
        </Flex>
      </div>
    </Flex>
  );
}
