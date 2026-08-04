import { DollarOutlined } from "@ant-design/icons";
import { Card } from "antd";
import { Link } from "react-router-dom";

import { useReceivableTotalToReceive } from "../../hooks/useReceivableTotalToReceive";
import { formatBRL } from "../../presentation/money";
import { LoadingState, UnavailableState } from "../AsyncState";

export function TotalReceivableCard() {
  const { total, isLoading, error, refetch } = useReceivableTotalToReceive();

  return (
    <Card
      className="dashboard-widget"
      title={
        <span className="dashboard-widget-title">
          <DollarOutlined aria-hidden="true" />
          Total a receber
        </span>
      }
      extra={<Link to="/contas-a-receber">Ver contas a receber</Link>}
    >
      {isLoading && <LoadingState>Carregando total a receber…</LoadingState>}
      {!isLoading && (error || !total) && (
        <UnavailableState onRetry={refetch}>
          Não foi possível carregar o total a receber.
        </UnavailableState>
      )}
      {!isLoading && !error && total && (
        <div className="total-debt-body">
          <p className="dashboard-hero-figure">{formatBRL(total.total_to_receive)}</p>
          {Number(total.total_to_receive) <= 0 && (
            <p className="dashboard-empty">Nenhuma conta a receber em aberto no momento.</p>
          )}
        </div>
      )}
    </Card>
  );
}
