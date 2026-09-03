import { Card, Tag, Tooltip, Typography } from "antd";

import type { Investment, InvestmentTransaction } from "../../api/contracts";
import { formatOptionalDate } from "../../presentation/dates";
import {
  investmentTypeLabel,
  investmentYield,
  netContributed,
  yieldUnavailable,
} from "../../presentation/investmentLabels";
import { formatBRL } from "../../presentation/money";

export function InvestmentHeaderCard({
  investment,
  transactions,
}: {
  investment: Investment;
  transactions: InvestmentTransaction[];
}) {
  const yieldEstimate = investmentYield(investment);
  const unavailable = yieldUnavailable(investment);
  // A history the backend judged too partial to net is too partial to show a
  // total for either. Without this the card contradicts itself: "Rendimento:
  // Histórico incompleto" directly above a confident "Aportes líquidos"
  // netted from the very history that was just rejected. The backend owns
  // that judgement — it has the quantity evidence (cotas sold against cotas
  // bought) this component does not.
  const historyIsTrustworthy = investment.yield_unavailable_reason !== "historico_incompleto";
  const contributed =
    transactions.length > 0 && historyIsTrustworthy ? netContributed(transactions) : null;

  return (
    <Card className="debt-header-card dashboard-widget">
      <div className="debt-header-identity">
        <Typography.Title level={3} className="debt-header-name">
          {investment.name ?? "Sem nome"}
        </Typography.Title>
        <Tag>{investmentTypeLabel(investment.investment_type)}</Tag>
        {investment.source_display_name !== null && (
          <Typography.Text type="secondary">{investment.source_display_name}</Typography.Text>
        )}
      </div>

      <div className="debt-header-figure">
        <p className="dashboard-hero-figure">
          {investment.balance !== null ? formatBRL(investment.balance) : "—"} de saldo
        </p>
        <ul className="debt-header-legend-inline">
          <li>
            <span>Rendimento</span>
            <Tooltip
              title={
                yieldEstimate === null
                  ? unavailable.hint === ""
                    ? undefined
                    : unavailable.hint
                  : yieldEstimate.source === "informado"
                    ? "Informado pela instituição"
                    : "Calculado a partir do histórico de aplicações e resgates"
              }
            >
              <strong>
                {yieldEstimate !== null ? formatBRL(yieldEstimate.value) : unavailable.label}
              </strong>
            </Tooltip>
          </li>
          {contributed !== null && (
            <li>
              <span>Aportes líquidos</span>
              <strong>{formatBRL(contributed)}</strong>
            </li>
          )}
          {investment.annual_rate !== null && (
            <li>
              <span>Taxa anual</span>
              <strong>{investment.annual_rate}%</strong>
            </li>
          )}
          {investment.last_twelve_months_rate !== null && (
            <li>
              <span>Rentabilidade 12 meses</span>
              <strong>{investment.last_twelve_months_rate}%</strong>
            </li>
          )}
          {investment.rate !== null && (
            <li>
              <span>Taxa contratada</span>
              <strong>
                {investment.rate}
                {investment.rate_type !== null ? ` ${investment.rate_type}` : ""}
              </strong>
            </li>
          )}
          {investment.issuer !== null && (
            <li>
              <span>Emissor</span>
              <strong>{investment.issuer}</strong>
            </li>
          )}
          {investment.due_date !== null && (
            <li>
              <span>Vencimento</span>
              <strong>{formatOptionalDate(investment.due_date)}</strong>
            </li>
          )}
          <li>
            <span>Atualizado em</span>
            <strong>{formatOptionalDate(investment.as_of_date)}</strong>
          </li>
        </ul>
      </div>
    </Card>
  );
}
