import { Skeleton } from "antd";
import { useMemo } from "react";

import { useTimeline } from "../../hooks/useTimeline";
import { useToday } from "../../hooks/useToday";
import { dailyAllowance } from "../../presentation/dailyAllowance";
import { UnavailableState } from "../AsyncState";
import { Section, SummaryStrip } from "../layout";
import { Money } from "../shared/Money";

const dateFormat = "YYYY-MM-DD";
const title = "Disponível por dia";

/**
 * What can be spent per day until month-end without the projected balance
 * going negative. Always this month from today, whatever period Home shows;
 * derived on every render from the timeline, never stored.
 */
export function DailyAllowanceCard() {
  const today = useToday();
  const referenceDate = today.format(dateFormat);
  const monthEnd = today.endOf("month").format(dateFormat);
  // from == referenceDate makes the server's lowest_balance the low point from
  // today (included) through month-end.
  const params = useMemo(
    () => ({ referenceDate, from: referenceDate, to: monthEnd }),
    [referenceDate, monthEnd],
  );
  const timeline = useTimeline(params);
  const base = timeline.base;

  if (timeline.isLoading || timeline.error || !base) {
    return (
      <Section title={title}>
        {timeline.isLoading && (
          <div role="status" aria-label="Carregando o gasto diário disponível">
            <Skeleton.Input active block style={{ height: 36 }} />
          </div>
        )}
        {!timeline.isLoading && (
          <UnavailableState onRetry={() => timeline.refetch()}>
            Não foi possível calcular o gasto diário disponível.
          </UnavailableState>
        )}
      </Section>
    );
  }

  return (
    <SummaryStrip
      className="daily-allowance"
      label={title}
      value={
        <>
          <Money value={dailyAllowance(base.lowest_balance.balance, today)} tone="neutral" size="hero" />
          <span className="daily-allowance-unit"> / dia</span>
        </>
      }
      note="Disponível para gastar até o fim do mês"
    />
  );
}
