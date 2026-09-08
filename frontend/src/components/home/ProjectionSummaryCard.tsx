import { LineChartOutlined } from "@ant-design/icons";
import { Card, Statistic } from "antd";
import dayjs from "dayjs";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import type { TimelineDayPoint } from "../../api/contracts";
import { LoadingState, UnavailableState } from "../AsyncState";
import { PeriodNavigator } from "../filters/PeriodNavigator";
import { periodNavigation } from "../filters/periodNavigation";
import { periodPresets } from "../filters/periodPresets";
import { ProjectionTimeline } from "../timeline/ProjectionTimeline";
import { useTimeline } from "../../hooks/useTimeline";
import { useTimelineDataRange } from "../../hooks/useTimelineDataRange";
import { formatDateOnly } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";
import { colors } from "../../theme/tokens";
import { WidgetCard } from "../shared/WidgetCard";

const dateFormat = "YYYY-MM-DD";

/** A window, in the same shape the transactions filter uses: nulls mean "all of it". */
type Period = { from: string | null; to: string | null };

const storageKey = "contadinho.home.saldo-periodo";
const defaultPreset = "this-year";

function presetPeriod(value: string): Period | null {
  const preset = periodPresets().find((candidate) => candidate.value === value);
  if (!preset) return null;
  const [from, to] = preset.range();
  return { from, to };
}

function defaultPeriod(): Period {
  return presetPeriod(defaultPreset) ?? { from: null, to: null };
}

function isDate(value: unknown): value is string {
  return typeof value === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value);
}

/**
 * The chosen window is a per-browser preference, not part of the page's
 * address: coming back to the Home through the menu must keep it, so it lives
 * in localStorage. Reading and writing it can throw outright — private
 * browsing, or a browser set to block site data — so neither side may be the
 * thing that breaks the dashboard.
 *
 * A window that came from a shortcut is stored as the shortcut and resolved
 * again on the way back in: "este ano" saved in December must still mean this
 * year in January, not the year it was picked.
 */
function storedPeriod(): Period {
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(storageKey);
  } catch {
    return defaultPeriod();
  }
  if (raw === null) return defaultPeriod();
  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) return defaultPeriod();
    const { preset, from, to } = parsed as Record<string, unknown>;
    if (typeof preset === "string") return presetPeriod(preset) ?? defaultPeriod();
    if (isDate(from) && isDate(to) && from <= to) return { from, to };
  } catch {
    // Anything unreadable — including the range names this key held before —
    // is simply not a window.
  }
  return defaultPeriod();
}

function storePeriod(period: Period): void {
  const preset = periodPresets().find((candidate) => {
    const [from, to] = candidate.range();
    return period.from === from && period.to === to;
  });
  try {
    window.localStorage.setItem(
      storageKey,
      JSON.stringify(preset ? { preset: preset.value } : period),
    );
  } catch {
    // A window that can't be remembered is still a usable window.
  }
}

function negativeValueStyle(value: string): { color: string } | undefined {
  return value.startsWith("-") ? { color: colors.error } : undefined;
}

function lowestPoint(points: TimelineDayPoint[]): TimelineDayPoint | null {
  return points.reduce<TimelineDayPoint | null>(
    (lowest, point) => (lowest === null || Number(point.balance) < Number(lowest.balance) ? point : lowest),
    null,
  );
}

// The dashboard is left open for hours, so "today" cannot be whatever it was
// when the tab was first painted: at midnight the balance anchor, and with it
// the whole window, must move to the new day.
function useToday(): dayjs.Dayjs {
  const [today, setToday] = useState(() => dayjs());
  useEffect(() => {
    const msToMidnight = today.add(1, "day").startOf("day").diff(dayjs());
    const timer = window.setTimeout(() => setToday(dayjs()), Math.max(msToMidnight, 0) + 1_000);
    return () => window.clearTimeout(timer);
  }, [today]);
  return today;
}

/**
 * The Home dashboard owns the balance view: it shows how the balance got to
 * today and where it is heading, while scenario editing remains available in
 * Cenários. The window is picked with the same control and the same shortcuts
 * as the transactions filter — the current year by default — so a period means
 * the same thing on both screens, even though each remembers its own.
 */
export function ProjectionSummaryCard() {
  const today = useToday();
  const navigate = useNavigate();
  const [period, setPeriodState] = useState<Period>(storedPeriod);
  const setPeriod = (from: string | null, to: string | null) => {
    setPeriodState({ from, to });
    storePeriod({ from, to });
  };

  // "Todo o período" is the one window whose bounds live in the database — the
  // oldest transaction, the last planned installment — so it stays unresolved
  // until useTimelineDataRange answers.
  const wholePeriod = period.from === null || period.to === null;
  const dataRange = useTimelineDataRange(wholePeriod);
  const bounds = useMemo(() => {
    if (!wholePeriod) return { from: period.from as string, to: period.to as string };
    return dataRange.range ? { from: dataRange.range.from, to: dataRange.range.to } : null;
  }, [wholePeriod, period.from, period.to, dataRange.range]);
  const referenceDate = today.format(dateFormat);
  const params = useMemo(
    () => ({
      // referenceDate is always today, never the start of the window: it is
      // what anchors the series to the real balance, so days before it read as
      // history and days after it as projection.
      referenceDate,
      from: bounds ? bounds.from : referenceDate,
      to: bounds ? bounds.to : referenceDate,
      // This widget plots only the balance curve; the monthly and per-category
      // breakdowns belong to the Relatório Financeiro.
      aggregations: false,
    }),
    [referenceDate, bounds],
  );
  const timeline = useTimeline(params, bounds !== null);
  const base = timeline.base;
  const isLoading = timeline.isLoading || dataRange.isLoading;
  const error = timeline.error ?? dataRange.error;
  const retry = () => {
    if (dataRange.error) dataRange.refetch();
    if (timeline.error) timeline.refetch();
  };
  const lastPoint = base && base.points.length > 0 ? base.points[base.points.length - 1] : null;
  const finalPoint = lastPoint && lastPoint.date > referenceDate ? lastPoint : null;
  // The server's lowest_balance deliberately ignores the past — a low balance
  // already lived through is no warning. A window that ends today has no
  // future to warn about, so there the card answers the question the period
  // actually poses: how low did it get?
  const lowest = base ? (finalPoint ? base.lowest_balance : lowestPoint(base.points)) : null;
  const lowestTitle = finalPoint ? "Menor saldo previsto em" : "Menor saldo no período em";

  return (
    <WidgetCard
      icon={<LineChartOutlined aria-hidden="true" />}
      title="Evolução do saldo"
      extra={
        <PeriodNavigator
          id="saldo-period"
          value={[period.from, period.to]}
          presets={periodPresets()}
          onChange={setPeriod}
          reset={{ preset: defaultPreset, label: "Este ano" }}
          bare
        />
      }
    >
      {isLoading && <LoadingState>Carregando o saldo…</LoadingState>}
      {error && !isLoading && (
        <UnavailableState onRetry={retry}>Não foi possível carregar o saldo.</UnavailableState>
      )}

      {!isLoading && !error && base && bounds && (
        <div className="projection-summary">
          <div className="projection-summary-intro">
            <div>
              {/* The header already names the chosen window, so spelling it
                  out again would only repeat it — except under "todo o
                  período", where the bounds come from the database and are
                  worth showing. */}
              {wholePeriod && (
                <p className="projection-summary-period">{periodNavigation(bounds.from, bounds.to)?.label}</p>
              )}
              <p className="projection-summary-note">
                {finalPoint
                  ? "Saldo realizado até hoje e, daí em diante, o que já está previsto — sem cenários hipotéticos. "
                  : "Saldo realizado, dia a dia. "}
                Clique num dia para ver seus lançamentos.
              </p>
            </div>
            <div className="projection-summary-stats">
              <Card size="small">
                <Statistic title="Saldo hoje" value={formatBRL(base.starting_balance)} />
              </Card>
              {lowest && (
                <Card size="small">
                  <Statistic
                    title={`${lowestTitle} ${formatDateOnly(lowest.date)}`}
                    value={formatBRL(lowest.balance)}
                    valueStyle={negativeValueStyle(lowest.balance)}
                  />
                </Card>
              )}
              {finalPoint && (
                <Card size="small">
                  <Statistic
                    title={`Saldo em ${formatDateOnly(finalPoint.date)}`}
                    value={formatBRL(finalPoint.balance)}
                    valueStyle={negativeValueStyle(finalPoint.balance)}
                  />
                </Card>
              )}
            </div>
          </div>
          <ProjectionTimeline
            series={base}
            referenceDate={referenceDate}
            height={280}
            onSelectDay={(date) => navigate(`/transacoes?date_from=${date}&date_to=${date}`)}
          />
        </div>
      )}
    </WidgetCard>
  );
}
