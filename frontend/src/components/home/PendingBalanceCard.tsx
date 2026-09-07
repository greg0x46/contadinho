import { RightOutlined, WalletOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";

import { useDebtTotalOwed } from "../../hooks/useDebtTotalOwed";
import { useReceivableTotalToReceive } from "../../hooks/useReceivableTotalToReceive";
import { formatBRL, subtractBRL, sumBRL } from "../../presentation/money";
import { SummaryCard } from "../shared/SummaryCard";

export function PendingBalanceCard() {
  const debt = useDebtTotalOwed();
  const receivable = useReceivableTotalToReceive();
  const owed = debt.total;
  const toReceive = receivable.total;
  const balance = owed && toReceive
    ? subtractBRL(toReceive.total_to_receive, owed.total_owed)
    : null;
  const inflow = Number(toReceive?.total_to_receive ?? 0);
  const outflow = Number(owed?.total_owed ?? 0);
  const inflowShare = inflow + outflow > 0 ? inflow / (inflow + outflow) * 100 : 0;
  const empty = owed && toReceive
    && sumBRL([owed.total_owed, toReceive.total_to_receive]) === "0.00";

  return (
    <SummaryCard
      icon={<WalletOutlined aria-hidden="true" />}
      title="Balanço"
      isLoading={debt.isLoading || receivable.isLoading}
      error={debt.error || receivable.error}
      hasData={Boolean(owed && toReceive)}
      loadingLabel="Carregando saldo das pendências…"
      errorLabel="Não foi possível carregar o saldo das pendências."
      onRetry={() => { void Promise.all([debt.refetch(), receivable.refetch()]); }}
    >
      {owed && toReceive && balance !== null && (
        <div className="pending-balance">
          <div>
            <p className="pending-balance-figure">
              {balance !== "0.00" && !balance.startsWith("-") ? "+" : ""}{formatBRL(balance)}
            </p>
            {empty && <p className="pending-balance-description">Nenhuma pendência em aberto</p>}
          </div>
          {!empty && (
            <div
              className="dashboard-meter pending-balance-meter"
              role="img"
              aria-label={`Entradas: ${formatBRL(toReceive.total_to_receive)}. Saídas (a pagar e cartão): ${formatBRL(owed.total_owed)}.`}
            >
              <span className="pending-balance-meter-inflow" style={{ width: `${inflowShare}%` }} />
              <span className="pending-balance-meter-outflow" style={{ width: `${100 - inflowShare}%` }} />
            </div>
          )}
          <div className="pending-balance-totals">
            <Link to="/pendencias?kind=receivable">
              <span>A receber</span>
              <strong>{formatBRL(toReceive.total_to_receive)}</strong>
              <RightOutlined aria-hidden="true" />
            </Link>
            <Link to="/pendencias?kind=debt">
              <span>A pagar</span>
              <strong>{formatBRL(owed.remaining_debts_total)}</strong>
              <RightOutlined aria-hidden="true" />
            </Link>
            <Link to="/transacoes?credit_card=true&card_balance=true&period=all">
              <span>Cartão de Crédito</span>
              <strong>{formatBRL(owed.future_installments_total)}</strong>
              <RightOutlined aria-hidden="true" />
            </Link>
          </div>
        </div>
      )}
    </SummaryCard>
  );
}
