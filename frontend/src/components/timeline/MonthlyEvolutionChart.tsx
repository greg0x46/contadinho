import { Bar, CartesianGrid, ComposedChart, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import type { MonthSummary } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { monthlyEvolutionColor } from "../../presentation/timelineLabels";

const monthNames = [
  "Jan",
  "Fev",
  "Mar",
  "Abr",
  "Mai",
  "Jun",
  "Jul",
  "Ago",
  "Set",
  "Out",
  "Nov",
  "Dez",
];

function monthLabel(value: string): string {
  const [, month] = value.split("-");
  return monthNames[Number(month) - 1] ?? value;
}

type ChartPoint = { month: string; income: number; expense: number; result: number };

function toChartData(summaries: MonthSummary[]): ChartPoint[] {
  return summaries.map((s) => ({
    month: monthLabel(s.month),
    income: Number(s.income),
    expense: Number(s.expense),
    result: Number(s.result),
  }));
}

function TooltipContent({ active, payload, label }: { active?: boolean; payload?: { value: number; dataKey: string }[]; label?: string }) {
  if (!active || !payload || payload.length === 0) return null;
  const byKey = Object.fromEntries(payload.map((p) => [p.dataKey, p.value]));
  return (
    <div className="timeline-chart-tooltip">
      <strong>{label}</strong>
      <div>Receitas: {formatBRL(String(byKey.income ?? 0))}</div>
      <div>Despesas: {formatBRL(String(byKey.expense ?? 0))}</div>
      <div>Resultado: {formatBRL(String(byKey.result ?? 0))}</div>
    </div>
  );
}

export function MonthlyEvolutionChart({ summaries }: { summaries: MonthSummary[] }) {
  const data = toChartData(summaries);
  return (
    <div className="timeline-chart" aria-label="Evolução mensal de receitas, despesas e resultado">
      <ResponsiveContainer width="100%" height={320}>
        <ComposedChart data={data} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="month" tickLine={false} axisLine={{ stroke: "#d9d9d9" }} />
          <YAxis tickLine={false} axisLine={false} width={0} />
          <Tooltip content={<TooltipContent />} />
          <Legend />
          <Bar dataKey="income" name="Receitas" fill={monthlyEvolutionColor.income} radius={[4, 4, 0, 0]} />
          <Bar dataKey="expense" name="Despesas" fill={monthlyEvolutionColor.expense} radius={[4, 4, 0, 0]} />
          <Line
            type="monotone"
            dataKey="result"
            name="Resultado"
            stroke={monthlyEvolutionColor.result}
            strokeWidth={2}
            dot={{ r: 4 }}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}
