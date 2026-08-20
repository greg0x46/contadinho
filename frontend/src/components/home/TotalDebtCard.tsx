import { WalletOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";

import type { PayableTotalOwed } from "../../api/contracts";
import { useDebtTotalOwed } from "../../hooks/useDebtTotalOwed";
import { formatBRL } from "../../presentation/money";
import { SummaryCard } from "../shared/SummaryCard";

function TotalDebtBreakdown({ total }: { total: PayableTotalOwed }) {
  const remaining = Number(total.remaining_debts_total);
  const future = Number(total.future_installments_total);
  const sum = remaining + future;

  if (sum <= 0) {
    return (
      <div className="total-debt-body">
        <p className="dashboard-hero-figure">{formatBRL(total.total_owed)}</p>
        <p className="dashboard-empty">Nenhuma dívida em aberto no momento.</p>
      </div>
    );
  }

  const segments = [
    { key: "remaining", share: (remaining / sum) * 100, className: "total-debt-meter-remaining" },
    { key: "future", share: (future / sum) * 100, className: "total-debt-meter-future" },
  ].filter((segment) => segment.share > 0);

  return (
    <div className="total-debt-body">
      <p className="dashboard-hero-figure">{formatBRL(total.total_owed)}</p>
      <div className="dashboard-meter" aria-hidden="true">
        {segments.map((segment) => (
          <span
            key={segment.key}
            className={`dashboard-meter-segment ${segment.className}`}
            style={{ width: `${segment.share}%` }}
          />
        ))}
      </div>
      <ul className="dashboard-legend">
        <li>
          <span className="dashboard-swatch total-debt-swatch-remaining" aria-hidden="true" />
          <span className="dashboard-legend-label">Dívidas restantes</span>
          <strong>{formatBRL(total.remaining_debts_total)}</strong>
        </li>
        <li>
          <span className="dashboard-swatch total-debt-swatch-future" aria-hidden="true" />
          <span className="dashboard-legend-label">Parcelas futuras (cartão)</span>
          <strong>{formatBRL(total.future_installments_total)}</strong>
        </li>
      </ul>
    </div>
  );
}

export function TotalDebtCard() {
  const { total, isLoading, error, refetch } = useDebtTotalOwed();

  return (
    <SummaryCard
      icon={<WalletOutlined aria-hidden="true" />}
      title="Dívida total"
      extra={<Link to="/pendencias?kind=debt">Ver dívidas</Link>}
      isLoading={isLoading}
      error={error}
      hasData={Boolean(total)}
      loadingLabel="Carregando total de dívida…"
      errorLabel="Não foi possível carregar o total de dívida."
      onRetry={refetch}
    >
      {total && <TotalDebtBreakdown total={total} />}
    </SummaryCard>
  );
}
