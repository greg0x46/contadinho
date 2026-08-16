import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Card } from "antd";

import type { MonthSummary } from "../../api/contracts";
import { formatBRL, sumBRL } from "../../presentation/money";
import { monthlyEvolutionColor } from "../../presentation/timelineLabels";

const monthNames = ["Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"];

function monthLabel(value: string): string {
  const [, month] = value.split("-");
  return monthNames[Number(month) - 1] ?? value;
}

function TooltipContent({ active, payload, label }: { active?: boolean; payload?: { value: number }[]; label?: string }) {
  if (!active || !payload || payload.length === 0) return null;
  return (
    <div className="timeline-chart-tooltip">
      <strong>{label}</strong>
      <div>Acumulado: {formatBRL(String(payload[0].value))}</div>
    </div>
  );
}

// AccumulatedResultCard shows the running Income-Expense result across the
// months already loaded (a calendar year), one point per month — never
// recomputed against a different set of months than SummaryCards' "Acumulado
// no ano", both fed by the same monthly_breakdown array from the API.
export function AccumulatedResultCard({ summaries }: { summaries: MonthSummary[] }) {
  const data = summaries.reduce<{ month: string; accumulated: number }[]>((acc, s, index) => {
    const accumulated = sumBRL(summaries.slice(0, index + 1).map((m) => m.result));
    acc.push({ month: monthLabel(s.month), accumulated: Number(accumulated) });
    return acc;
  }, []);

  return (
    <Card size="small" title="Resultado acumulado no ano" className="timeline-chart">
      <ResponsiveContainer width="100%" height={220}>
        <LineChart data={data} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="month" tickLine={false} axisLine={{ stroke: "#d9d9d9" }} />
          <YAxis tickLine={false} axisLine={false} width={0} />
          <Tooltip content={<TooltipContent />} />
          <Line
            type="monotone"
            dataKey="accumulated"
            name="Acumulado"
            stroke={monthlyEvolutionColor.result}
            strokeWidth={2}
            dot={{ r: 4 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </Card>
  );
}
