import { Alert } from "antd";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { TimelineSeries } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { monthlyEvolutionColor } from "../../presentation/timelineLabels";
import { colors } from "../../theme/tokens";

function formatDate(value: string): string {
  const [year, month, day] = value.split("-");
  return `${day}/${month}/${year}`;
}

function TooltipContent({ active, payload, label }: { active?: boolean; payload?: { value: number; name: string }[]; label?: string }) {
  if (!active || !payload || payload.length === 0 || !label) return null;
  return (
    <div className="timeline-chart-tooltip">
      <strong>{formatDate(label)}</strong>
      {payload.map((entry) => (
        <div key={entry.name}>
          {entry.name}: {formatBRL(String(entry.value))}
        </div>
      ))}
    </div>
  );
}

type PointRow = { date: string; balance: number; simulationBalance?: number };

function mergePoints(base: TimelineSeries, simulation: TimelineSeries | null, referenceDate: string): PointRow[] {
  const rows = base.points.map((p) => ({
    date: p.date,
    // starting_balance is the canonical balance at the reference date. Keep
    // today's plotted point anchored to the same value as the summary card;
    // the selected horizon must only change points after today.
    balance: p.date === referenceDate ? Number(base.starting_balance) : Number(p.balance),
  }) as PointRow);
  if (!simulation) return rows;
  const simulationByDate = new Map(simulation.points.map((p) => [p.date, Number(p.balance)]));
  return rows.map((row) => ({
    ...row,
    simulationBalance:
      row.date === referenceDate ? Number(simulation.starting_balance) : simulationByDate.get(row.date),
  }));
}

// ProjectionTimeline draws Series.Points as one continuous line —
// Realizado→Confirmado→Projetado→Hipotético — never a separate chart per
// tier, since the balance itself is one continuous number regardless of
// how certain each contributing entry is. When a simulation is active, a
// second line joins it — always Base vs. Simulação as a whole, never one
// line per scenario even with several active (seção 20).
export function ProjectionTimeline({
  series,
  simulation,
  referenceDate,
  height = 280,
}: {
  series: TimelineSeries;
  simulation?: TimelineSeries | null;
  referenceDate: string;
  height?: number;
}) {
  const data = mergePoints(series, simulation ?? null, referenceDate);
  const todayPoint = series.points.find((p) => p.date === referenceDate);
  const activeSeries = simulation ?? series;

  return (
    <div className="timeline-chart" aria-label="Projeção de saldo ao longo do tempo">
      {activeSeries.first_negative && (
        <Alert
          type="warning"
          showIcon
          message={`Se nada mudar, o saldo pode ficar negativo em ${formatDate(activeSeries.first_negative)}.`}
          style={{ marginBottom: 12 }}
        />
      )}
      <ResponsiveContainer width="100%" height={height}>
        <LineChart data={data} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="date" tickFormatter={formatDate} tickLine={false} axisLine={{ stroke: colors.border }} />
          <YAxis tickLine={false} axisLine={false} width={0} />
          <Tooltip content={<TooltipContent />} />
          {simulation && <Legend />}
          {todayPoint && (
            <ReferenceLine x={todayPoint.date} stroke={colors.textSecondary} strokeDasharray="4 4" label="Hoje" />
          )}
          <ReferenceDot
            x={activeSeries.lowest_balance.date}
            y={Number(activeSeries.lowest_balance.balance)}
            r={5}
            fill={monthlyEvolutionColor.expense}
            stroke="none"
            label={{ value: "Menor saldo", position: "top" }}
          />
          <Line
            type="stepAfter"
            dataKey="balance"
            name="Saldo (base)"
            stroke={monthlyEvolutionColor.result}
            strokeWidth={2}
            dot={false}
          />
          {simulation && (
            <Line
              type="stepAfter"
              dataKey="simulationBalance"
              name="Saldo (simulação)"
              stroke={monthlyEvolutionColor.income}
              strokeWidth={2}
              strokeDasharray="4 4"
              dot={false}
            />
          )}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
