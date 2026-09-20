import { LineChartOutlined } from "@ant-design/icons";
import { Card, Statistic } from "antd";
import dayjs from "dayjs";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import type { TimelineDayPoint } from "../../api/contracts";
import { LoadingState, UnavailableState } from "../AsyncState";
import type { HomePeriod } from "../../hooks/useHomePeriod";
import { periodNavigation } from "../filters/periodNavigation";
import { ProjectionTimeline } from "../timeline/ProjectionTimeline";
import { useHomePeriodBounds } from "../../hooks/useHomePeriodBounds";
import { useTimeline } from "../../hooks/useTimeline";
import { formatDateOnly } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";
import { colors } from "../../theme/tokens";
import { WidgetCard } from "../shared/WidgetCard";

const dateFormat = "YYYY-MM-DD";

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

export function ProjectionSummaryCard({ period }: { period: HomePeriod }) {
  const today = useToday();
  const navigate = useNavigate();

  // "Todo o período" is the one window whose bounds live in the database — the
  // oldest transaction, the last planned installment — so it stays unresolved
  // until useHomePeriodBounds answers.
  const { bounds, wholePeriod, isLoading: rangeLoading, error: rangeError, refetch: refetchRange } =
    useHomePeriodBounds(period);
  const referenceDate = today.format(dateFormat);
  const params = useMemo(
    () => ({
      // referenceDate is always today, never the start of the window: it is
      // what anchors the series to the real balance, so days before it read as
      // history and days after it as projection.
      referenceDate,
      from: bounds ? bounds.from : referenceDate,
      to: bounds ? bounds.to : referenceDate,
    }),
    [referenceDate, bounds],
  );
  const timeline = useTimeline(params, bounds !== null);
  const base = timeline.base;
  const isLoading = timeline.isLoading || rangeLoading;
  const error = timeline.error ?? rangeError;
  const retry = () => {
    if (rangeError) refetchRange();
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
    >
      {isLoading && <LoadingState>Carregando o saldo…</LoadingState>}
      {error && !isLoading && (
        <UnavailableState onRetry={retry}>Não foi possível carregar o saldo.</UnavailableState>
      )}

      {!isLoading && !error && base && bounds && (
        <div className="projection-summary">
          <div className="projection-summary-intro">
            <div>
              {/* The global control already names the chosen window, so spelling it
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
