import { LineChartOutlined } from "@ant-design/icons";
import { Card, Segmented, Statistic } from "antd";
import dayjs from "dayjs";
import { useMemo, useState } from "react";

import { LoadingState, UnavailableState } from "../AsyncState";
import { ProjectionTimeline } from "../timeline/ProjectionTimeline";
import { useTimeline } from "../../hooks/useTimeline";
import { formatDateOnly } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";
import { colors } from "../../theme/tokens";
import { WidgetCard } from "../shared/WidgetCard";

const dateFormat = "YYYY-MM-DD";

const horizons = [
  { label: "Fim do mês", value: "month" },
  { label: "3 meses", value: "3m" },
  { label: "6 meses", value: "6m" },
  { label: "12 meses", value: "12m" },
] as const;

type Horizon = (typeof horizons)[number]["value"];

function horizonEnd(today: dayjs.Dayjs, horizon: Horizon): dayjs.Dayjs {
  switch (horizon) {
    case "month":
      return today.endOf("month");
    case "3m":
      return today.add(3, "month").endOf("month");
    case "6m":
      return today.add(6, "month").endOf("month");
    case "12m":
      return today.add(12, "month").endOf("month");
  }
}

function horizonLabel(horizon: Horizon): string {
  return horizon === "month" ? "Até o fim do mês" : `Próximos ${horizon.replace("m", " meses")}`;
}

function negativeValueStyle(value: string): { color: string } | undefined {
  return value.startsWith("-") ? { color: colors.error } : undefined;
}

/**
 * The Home dashboard owns the projection view: it gives a quick answer about
 * the near future while scenario editing remains available in Cenários. It
 * keeps the same horizon options that were available in the former projection
 * page, with three months selected by default.
 */
export function ProjectionSummaryCard() {
  const [today] = useState(() => dayjs());
  const [horizon, setHorizon] = useState<Horizon>("3m");
  const params = useMemo(
    () => ({
      referenceDate: today.format(dateFormat),
      from: today.format(dateFormat),
      to: horizonEnd(today, horizon).format(dateFormat),
    }),
    [today, horizon],
  );
  const timeline = useTimeline(params);
  const base = timeline.base;
  const finalPoint = base && base.points.length > 0 ? base.points[base.points.length - 1] : null;

  return (
    <WidgetCard
      icon={<LineChartOutlined aria-hidden="true" />}
      title="Projeção de saldo"
      extra={
        <Segmented
          className="projection-summary-horizon"
          options={[...horizons]}
          value={horizon}
          onChange={(value) => setHorizon(value as Horizon)}
          aria-label="Horizonte da projeção"
        />
      }
    >
      {timeline.isLoading && <LoadingState>Carregando projeção…</LoadingState>}
      {timeline.error && !timeline.isLoading && (
        <UnavailableState onRetry={() => timeline.refetch()}>
          Não foi possível carregar a projeção.
        </UnavailableState>
      )}

      {!timeline.isLoading && !timeline.error && base && (
        <div className="projection-summary">
          <div className="projection-summary-intro">
            <div>
              <p className="projection-summary-period">{horizonLabel(horizon)}</p>
              <p className="projection-summary-note">
                Considerando o que já está previsto, sem cenários hipotéticos.
              </p>
            </div>
            <div className="projection-summary-stats">
              <Card size="small">
                <Statistic title="Saldo hoje" value={formatBRL(base.starting_balance)} />
              </Card>
              <Card size="small">
                <Statistic
                  title={`Menor saldo em ${formatDateOnly(base.lowest_balance.date)}`}
                  value={formatBRL(base.lowest_balance.balance)}
                  valueStyle={negativeValueStyle(base.lowest_balance.balance)}
                />
              </Card>
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
          <ProjectionTimeline series={base} referenceDate={params.referenceDate} height={280} />
        </div>
      )}
    </WidgetCard>
  );
}
