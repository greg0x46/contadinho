import { DollarOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";

import { useReceivableTotalToReceive } from "../../hooks/useReceivableTotalToReceive";
import { formatBRL } from "../../presentation/money";
import { SummaryCard } from "../shared/SummaryCard";

export function TotalReceivableCard() {
  const { total, isLoading, error, refetch } = useReceivableTotalToReceive();

  return (
    <SummaryCard
      icon={<DollarOutlined aria-hidden="true" />}
      title="Total a receber"
      extra={<Link to="/pendencias?kind=receivable">Ver contas a receber</Link>}
      isLoading={isLoading}
      error={error}
      hasData={Boolean(total)}
      loadingLabel="Carregando total a receber…"
      errorLabel="Não foi possível carregar o total a receber."
      onRetry={refetch}
    >
      {total && (
        <div className="total-debt-body">
          <p className="dashboard-hero-figure">{formatBRL(total.total_to_receive)}</p>
          {Number(total.total_to_receive) <= 0 && (
            <p className="dashboard-empty">Nenhuma conta a receber em aberto no momento.</p>
          )}
        </div>
      )}
    </SummaryCard>
  );
}
