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

import type { TimelineEntry, TimelineSeries } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { monthlyEvolutionColor } from "../../presentation/chartColors";
import { colors } from "../../theme/tokens";

function formatDate(value: string): string {
  const [year, month, day] = value.split("-");
  return `${day}/${month}/${year}`;
}

const pastLineName = "Saldo realizado";
const futureLineName = "Saldo projetado";

// The past and future halves of the base line share the reference day, so
// without this the tooltip would list the same balance twice on that one day.
function tooltipRows(payload: { value: number; name: string }[]): { value: number; name: string }[] {
  let baseShown = false;
  return payload.filter((entry) => {
    if (entry.value === undefined || entry.value === null) return false;
    if (entry.name !== pastLineName && entry.name !== futureLineName) return true;
    if (baseShown) return false;
    baseShown = true;
    return true;
  });
}

// A balance alone doesn't explain itself: the day it drops, what the reader
// wants is which entries moved it. Only a handful fit before the tooltip
// becomes a wall, so the largest ones win and the rest are counted.
const maxTooltipEntries = 4;

function DayEntries({ entries }: { entries: TimelineEntry[] }) {
  if (entries.length === 0) return null;
  const ranked = [...entries].sort((a, b) => Math.abs(Number(b.amount)) - Math.abs(Number(a.amount)));
  const shown = ranked.slice(0, maxTooltipEntries);
  const hidden = ranked.length - shown.length;
  return (
    <ul className="timeline-chart-tooltip-entries">
      {shown.map((entry) => (
        <li key={`${entry.source}-${entry.source_ref_id}-${entry.description}-${entry.amount}`}>
          <span>{entry.description}</span>
          <span style={{ color: Number(entry.amount) < 0 ? colors.error : undefined }}>
            {formatBRL(entry.amount)}
          </span>
        </li>
      ))}
      {hidden > 0 && <li className="timeline-chart-tooltip-more">e mais {hidden}</li>}
    </ul>
  );
}

function TooltipContent({
  active,
  payload,
  label,
  entriesByDate,
}: {
  active?: boolean;
  payload?: { value: number; name: string }[];
  label?: string;
  entriesByDate?: Map<string, TimelineEntry[]>;
}) {
  if (!active || !payload || payload.length === 0 || !label) return null;
  const rows = tooltipRows(payload);
  if (rows.length === 0) return null;
  return (
    <div className="timeline-chart-tooltip">
      <strong>{formatDate(label)}</strong>
      {rows.map((entry) => (
        <div key={entry.name}>
          {entry.name}: {formatBRL(String(entry.value))}
        </div>
      ))}
      <DayEntries entries={entriesByDate?.get(label) ?? []} />
    </div>
  );
}

// The Y axis exists to give the curve a scale, not to be read to the cent —
// so its ticks are rounded to thousands, keeping the axis narrow.
function formatAxisMoney(value: number): string {
  const abs = Math.abs(value);
  const sign = value < 0 ? "-" : "";
  if (abs >= 1_000_000) return `${sign}R$\u00a0${(abs / 1_000_000).toFixed(1).replace(".", ",")} mi`;
  if (abs >= 1_000) return `${sign}R$\u00a0${Math.round(abs / 1_000)} mil`;
  return `${sign}R$\u00a0${Math.round(abs)}`;
}

// A window can span a full year of daily points, where "dd/mm/yyyy" ticks
// collide; past roughly four months only the month is worth reading.
function tickFormatterFor(pointCount: number): (value: string) => string {
  if (pointCount <= 120) return formatDate;
  return (value: string) => {
    const [year, month] = value.split("-");
    return `${month}/${year}`;
  };
}

type PointRow = {
  date: string;
  balance: number;
  pastBalance?: number;
  futureBalance?: number;
  simulationBalance?: number;
};

function mergePoints(base: TimelineSeries, simulation: TimelineSeries | null, referenceDate: string): PointRow[] {
  const rows = base.points.map((p) => {
    // starting_balance is the canonical balance at the reference date. Keep
    // today's plotted point anchored to the same value as the summary card;
    // the selected window must only change points after today.
    const balance = p.date === referenceDate ? Number(base.starting_balance) : Number(p.balance);
    // The reference day belongs to both halves so the solid past line and the
    // dashed future line meet instead of leaving a gap.
    return {
      date: p.date,
      balance,
      pastBalance: p.date <= referenceDate ? balance : undefined,
      futureBalance: p.date >= referenceDate ? balance : undefined,
    } as PointRow;
  });
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
// how certain each contributing entry is. The only split is at the
// reference date, where the same line turns dashed: everything before it
// already happened, everything after it is a forecast. When a simulation
// is active, a second line joins it — always Base vs. Simulação as a
// whole, never one line per scenario even with several active (seção 20).
export function ProjectionTimeline({
  series,
  simulation,
  referenceDate,
  height = 280,
  onSelectDay,
}: {
  series: TimelineSeries;
  simulation?: TimelineSeries | null;
  referenceDate: string;
  height?: number;
  /**
   * Called with the clicked day. Only days up to the reference date are
   * reported: later ones are projections with no transaction behind them yet.
   */
  onSelectDay?: (date: string) => void;
}) {
  const data = mergePoints(series, simulation ?? null, referenceDate);
  const todayPoint = series.points.find((p) => p.date === referenceDate);
  const activeSeries = simulation ?? series;
  const hasPast = data.some((row) => row.date < referenceDate);
  // A window with no future day at all still gets the forward line, so a
  // one-point series is never drawn as nothing.
  const hasFuture = data.some((row) => row.date > referenceDate) || !hasPast;
  const entriesByDate = new Map<string, TimelineEntry[]>();
  for (const entry of activeSeries.entries) {
    const day = entriesByDate.get(entry.date);
    if (day) day.push(entry);
    else entriesByDate.set(entry.date, [entry]);
  }
  const handleClick = (state: { activeLabel?: string | number }) => {
    const date = state?.activeLabel;
    if (onSelectDay && typeof date === "string" && date <= referenceDate) onSelectDay(date);
  };

  return (
    <div className="timeline-chart" aria-label="Saldo ao longo do tempo">
      {activeSeries.first_negative && (
        <Alert
          type="warning"
          showIcon
          message={`Se nada mudar, o saldo pode ficar negativo em ${formatDate(activeSeries.first_negative)}.`}
          style={{ marginBottom: 12 }}
        />
      )}
      <ResponsiveContainer width="100%" height={height}>
        <LineChart
          data={data}
          margin={{ top: 8, right: 8, left: 8, bottom: 8 }}
          onClick={onSelectDay ? handleClick : undefined}
          style={onSelectDay ? { cursor: "pointer" } : undefined}
        >
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="date"
            tickFormatter={tickFormatterFor(data.length)}
            interval="preserveStartEnd"
            tickLine={false}
            axisLine={{ stroke: colors.border }}
          />
          <YAxis
            tickFormatter={formatAxisMoney}
            tickLine={false}
            axisLine={false}
            width={72}
            tick={{ fontSize: 12 }}
          />
          <Tooltip content={<TooltipContent entriesByDate={entriesByDate} />} />
          <Legend />
          {/* Zero is the line that matters on this chart — where the balance
              crosses it is the whole point of looking. */}
          <ReferenceLine y={0} stroke={colors.error} strokeOpacity={0.5} />
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
          {hasPast && (
            <Line
              type="stepAfter"
              dataKey="pastBalance"
              name={pastLineName}
              stroke={monthlyEvolutionColor.result}
              strokeWidth={2}
              dot={false}
              connectNulls={false}
            />
          )}
          {hasFuture && (
            <Line
              type="stepAfter"
              dataKey="futureBalance"
              name={futureLineName}
              stroke={monthlyEvolutionColor.result}
              strokeWidth={2}
              strokeDasharray="4 4"
              dot={false}
              connectNulls={false}
            />
          )}
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
