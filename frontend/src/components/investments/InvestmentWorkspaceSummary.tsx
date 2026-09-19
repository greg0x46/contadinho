import { Alert, Card } from "antd";

import type { InvestmentOperation, InvestmentSummary } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";
import { InvestmentFigures } from "./InvestmentFigures";
import { accountMovements } from "./investmentFigures";

export function InvestmentWorkspaceSummary({
  summary,
  operations,
}: {
  summary: InvestmentSummary | null;
  operations: InvestmentOperation[];
}) {
  const movements = accountMovements(operations);

  return (
    <Card title="Total investido" style={{ marginBottom: 16 }}>
      <InvestmentFigures
        figures={[
          { label: "Valor atual", value: formatBRL(summary?.total_value ?? "0") },
          { label: "Aportes", value: formatBRL(movements.deposits) },
          { label: "Resgates", value: formatBRL(movements.withdrawals) },
          {
            label: "Caixa disponível para investir",
            value: formatBRL(summary?.cash_balance ?? "0"),
            hint: "Já saiu da conta corrente, ainda não foi aplicado em nenhuma posição.",
          },
          {
            label: "Informado pela instituição",
            value: formatBRL(summary?.synced_value ?? "0"),
            hint: "Parte do total que vem da integração bancária; o restante é registro manual.",
          },
        ]}
      />
      <Alert
        type="info"
        showIcon
        message="Objetivos apenas agrupam valor"
        description="Um objetivo mostra quanto das suas posições está reservado para ele. Ele não soma nada ao total investido nem ao patrimônio, e mudar o objetivo de uma posição não move dinheiro."
      />
    </Card>
  );
}
