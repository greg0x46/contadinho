import { Card, Flex, Statistic } from "antd";

import type { TimelineSeries } from "../../api/contracts";
import { formatBRL, sumBRL } from "../../presentation/money";

function finalBalance(series: TimelineSeries): string {
  if (series.points.length === 0) return series.starting_balance;
  return series.points[series.points.length - 1].balance;
}

function negate(value: string): string {
  return value.startsWith("-") ? value.slice(1) : `-${value}`;
}

export function BaseVsSimulationCompare({
  base,
  simulation,
}: {
  base: TimelineSeries;
  simulation: TimelineSeries;
}) {
  const baseFinal = finalBalance(base);
  const simulationFinal = finalBalance(simulation);
  const impact = sumBRL([simulationFinal, negate(baseFinal)]);

  return (
    <Card size="small" title="Base × Simulação" className="timeline-chart">
      <Flex gap="middle" wrap>
        <Statistic title="Saldo projetado (base)" value={formatBRL(baseFinal)} />
        <Statistic title="Saldo projetado (com cenários)" value={formatBRL(simulationFinal)} />
        <Statistic
          title="Impacto combinado"
          value={formatBRL(impact)}
          valueStyle={{ color: impact.startsWith("-") ? "#e34948" : "#008300" }}
        />
      </Flex>
    </Card>
  );
}
