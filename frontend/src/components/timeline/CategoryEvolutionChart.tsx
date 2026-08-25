import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Card } from "antd";

import type { MonthAmount } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { monthlyEvolutionColor } from "../../presentation/chartColors";
import { colors } from "../../theme/tokens";

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
      <div>{formatBRL(String(payload[0].value))}</div>
    </div>
  );
}

// CategoryEvolutionChart reads timeline_evolution — the same Series
// CategoryImpactList's numbers come from, grouped by month for one
// category instead of by category for one month — never a second query.
export function CategoryEvolutionChart({
  categoryName,
  months,
  onClose,
}: {
  categoryName: string;
  months: MonthAmount[];
  onClose: () => void;
}) {
  const data = months.map((m) => ({ month: monthLabel(m.month), amount: Number(m.amount) }));
  return (
    <Card
      size="small"
      title={`Evolução de ${categoryName}`}
      extra={
        <button type="button" className="timeline-scenario-clear" onClick={onClose}>
          Fechar
        </button>
      }
      className="timeline-chart"
    >
      <ResponsiveContainer width="100%" height={220}>
        <LineChart data={data} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="month" tickLine={false} axisLine={{ stroke: colors.border }} />
          <YAxis tickLine={false} axisLine={false} width={0} />
          <Tooltip content={<TooltipContent />} />
          <Line
            type="monotone"
            dataKey="amount"
            name={categoryName}
            stroke={monthlyEvolutionColor.expense}
            strokeWidth={2}
            dot={{ r: 4 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </Card>
  );
}
