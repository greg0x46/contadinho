import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import type { NetWorthSnapshot } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { monthlyEvolutionColor } from "../../presentation/timelineLabels";

type ChartPoint = { date: string; netWorth: number };

function dateLabel(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" });
}

function toChartData(snapshots: NetWorthSnapshot[]): ChartPoint[] {
  return snapshots.map((s) => ({ date: dateLabel(s.captured_at), netWorth: Number(s.net_worth) }));
}

function TooltipContent({ active, payload, label }: { active?: boolean; payload?: { value: number }[]; label?: string }) {
  if (!active || !payload || payload.length === 0) return null;
  return (
    <div className="timeline-chart-tooltip">
      <strong>{label}</strong>
      <div>Patrimônio líquido: {formatBRL(String(payload[0].value))}</div>
    </div>
  );
}

// A single series never needs a legend (see the dataviz skill) — the chart
// title alone names what the line is. The color reuses
// monthlyEvolutionColor.result (already validated for CVD/contrast) since
// "net worth" is the same semantic role as "result" in the monthly
// evolution chart: a signed, netted-out figure.
export function NetWorthChart({ snapshots }: { snapshots: NetWorthSnapshot[] }) {
  const data = toChartData(snapshots);
  return (
    <div className="timeline-chart" aria-label="Evolução do patrimônio líquido">
      <ResponsiveContainer width="100%" height={320}>
        <LineChart data={data} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="date" tickLine={false} axisLine={{ stroke: "#d9d9d9" }} />
          <YAxis tickLine={false} axisLine={false} width={0} />
          <Tooltip content={<TooltipContent />} />
          <Line
            type="monotone"
            dataKey="netWorth"
            name="Patrimônio líquido"
            stroke={monthlyEvolutionColor.result}
            strokeWidth={2}
            dot={{ r: 4 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
