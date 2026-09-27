import { FundOutlined } from "@ant-design/icons";
import { Skeleton, Tooltip } from "antd";

import type { Investment } from "../../api/contracts";
import { investmentTypeLabel, investmentYield, yieldUnavailable } from "../../presentation/investmentLabels";
import { formatBRL } from "../../presentation/money";
import { colors } from "../../theme/tokens";

function YieldFigure({ investment }: { investment: Investment }) {
  const estimate = investmentYield(investment);
  if (estimate === null) {
    const { label, hint } = yieldUnavailable(investment);
    return (
      <Tooltip title={hint === "" ? undefined : hint}>
        <span className="investment-row-figure-value">{label}</span>
      </Tooltip>
    );
  }
  const negative = estimate.value.startsWith("-");
  return (
    <Tooltip
      title={
        estimate.source === "informado"
          ? "Informado pela instituição"
          : "Calculado a partir do histórico de aplicações e resgates"
      }
    >
      <span
        className="investment-row-figure-value"
        style={{ color: negative ? colors.error : colors.success }}
      >
        {formatBRL(estimate.value)}
      </span>
    </Tooltip>
  );
}

/**
 * Investimentos sincronizados as flat rows: name + institution/type on the
 * left, saldo atual and rendimento as the two figures on the right — the
 * same recipe as BankAccountList/CreditCardList on Contas e cartões, instead
 * of a ProTable grid.
 */
export function InvestmentList({
  investments,
  isLoading,
  onOpen,
}: {
  investments: Investment[];
  isLoading: boolean;
  onOpen: (investment: Investment) => void;
}) {
  return (
    <section className="investments-section" aria-label="Investimentos sincronizados">
      <header className="investments-section-header">
        <h2>Investimentos sincronizados</h2>
        {!isLoading && investments.length > 0 && (
          <small>
            {investments.length} {investments.length === 1 ? "investimento" : "investimentos"}
          </small>
        )}
      </header>
      {isLoading ? (
        <div className="investments-section-loading" role="status" aria-label="Carregando investimentos">
          <Skeleton active paragraph={{ rows: 2 }} title={false} />
        </div>
      ) : investments.length === 0 ? (
        <div className="debt-list-empty">
          <FundOutlined className="debt-list-empty-icon" aria-hidden="true" />
          <span>Nenhum investimento sincronizado ainda.</span>
        </div>
      ) : (
        <div className="investments-section-rows">
          {investments.map((investment) => (
            <button
              key={investment.id}
              type="button"
              className="investment-row"
              onClick={() => onOpen(investment)}
            >
              <span className="investment-row-identity">
                <span className="investment-row-name">{investment.name ?? "Sem nome"}</span>
                <span className="investment-row-meta">
                  {[investment.source_display_name, investmentTypeLabel(investment.investment_type)]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </span>
              <span className="investment-row-figures">
                <span className="investment-row-figure">
                  <span className="investment-row-figure-label">Saldo atual</span>
                  <span className="investment-row-figure-value">
                    {investment.balance !== null ? formatBRL(investment.balance) : "—"}
                  </span>
                </span>
                <span className="investment-row-figure">
                  <span className="investment-row-figure-label">Rendimento</span>
                  <YieldFigure investment={investment} />
                </span>
              </span>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
