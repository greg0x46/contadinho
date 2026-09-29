import { Typography } from "antd";
import type { InvestmentOperation } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

export function InvestmentRedemptionBreakdown({ operation }: { operation: InvestmentOperation }) {
  if (operation.kind !== "redemption") return null;
  return (
    <Typography.Text type="secondary">
      Aporte {formatBRL(operation.principal_amount)} · Rendimento bruto {formatBRL(operation.income_amount)}
      {" · "}Taxas {formatBRL(operation.fees ?? "0")} · Impostos {formatBRL(operation.taxes ?? "0")}
      {" · "}Líquido {formatBRL(operation.amount)}
    </Typography.Text>
  );
}
