import { WalletOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { useMemo } from "react";
import { Link } from "react-router-dom";

import type { Period } from "../../hooks/usePeriod";
import { useHomePeriodBounds } from "../../hooks/useHomePeriodBounds";
import { useTimeline } from "../../hooks/useTimeline";
import { formatBRL } from "../../presentation/money";
import { SummaryCard } from "../shared/SummaryCard";

const dateFormat = "YYYY-MM-DD";

export function PeriodBalanceCard({ period }: { period: Period }) {
  const { bounds, isLoading: rangeLoading, error: rangeError, refetch: refetchRange } =
    useHomePeriodBounds(period);
  // Only the day span matters here, not the exact instant, so this needs
  // none of ProjectionSummaryCard's midnight-rollover handling.
  const referenceDate = useMemo(() => dayjs().format(dateFormat), []);
  const params = useMemo(
    () => ({
      referenceDate,
      from: bounds ? bounds.from : referenceDate,
      to: bounds ? bounds.to : referenceDate,
    }),
    [referenceDate, bounds],
  );
  const timeline = useTimeline(params, bounds !== null);
  // periodTotals — not the daily points — is what matches the transactions
  // page: it walks every real (and, with active scenarios, projected) entry
  // in the window, the same income/expense basis Query's totals use
  // (Included, not just MovesCash), so a same-owner transfer or a settled
  // credit-card purchase is counted exactly once, on the right side. The
  // points, by contrast, drop a settled purchase entirely once its bill is
  // paid (see internal/timeline's cashEntries) — correct for the balance
  // curve, wrong for an entradas/saídas total.
  const totals = timeline.periodTotals;
  const inflow = totals?.income ?? null;
  const outflow = totals?.expense ?? null;
  const balance = totals?.result ?? null;
  const inflowNumber = Number(inflow ?? 0);
  const outflowNumber = Number(outflow ?? 0);
  const inflowShare = inflowNumber + outflowNumber > 0 ? (inflowNumber / (inflowNumber + outflowNumber)) * 100 : 0;
  const empty = inflow === "0.00" && outflow === "0.00";
  const isLoading = timeline.isLoading || rangeLoading;
  const error = timeline.error ?? rangeError;

  return (
    <SummaryCard
      icon={<WalletOutlined aria-hidden="true" />}
      title="Saldo do período"
      isLoading={isLoading}
      error={error}
      hasData={balance !== null && bounds !== null}
      loadingLabel="Carregando o saldo do período…"
      errorLabel="Não foi possível carregar o saldo do período."
      onRetry={() => {
        if (rangeError) refetchRange();
        if (timeline.error) timeline.refetch();
      }}
    >
      {bounds && balance !== null && inflow !== null && outflow !== null && (
        <div className="period-balance">
          <p className="period-balance-description">Entradas menos saídas, considerando os cenários ativos</p>
          <div>
            <p className="period-balance-figure">
              {balance !== "0.00" && !balance.startsWith("-") ? "+" : ""}{formatBRL(balance)}
            </p>
            {empty && <p className="period-balance-description">Nenhuma movimentação no período</p>}
          </div>
          {!empty && (
            <div
              className="dashboard-meter period-balance-meter"
              role="img"
              aria-label={`Entradas: ${formatBRL(inflow)}. Saídas: ${formatBRL(outflow)}.`}
            >
              <span className="period-balance-meter-inflow" style={{ width: `${inflowShare}%` }} />
              <span className="period-balance-meter-outflow" style={{ width: `${100 - inflowShare}%` }} />
            </div>
          )}
          <div className="period-balance-totals">
            <Link to={`/transacoes?date_from=${bounds.from}&date_to=${bounds.to}&classification=inflow`}>
              <span>Entradas</span>
              <strong>{formatBRL(inflow)}</strong>
            </Link>
            <Link to={`/transacoes?date_from=${bounds.from}&date_to=${bounds.to}&classification=outflow`}>
              <span>Saídas</span>
              <strong>{formatBRL(outflow)}</strong>
            </Link>
          </div>
        </div>
      )}
    </SummaryCard>
  );
}
