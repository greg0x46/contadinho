import type { InvestmentOperation, InvestmentSummary } from "../../api/contracts";
import { Money } from "../shared/Money";
import { SummaryStrip } from "../layout";
import { accountMovements } from "./investmentMath";

/**
 * The page's hero figure — total value across every investment account —
 * with aportes, resgates, saldo para investir and o quanto vem de
 * instituições as secondary figures. Aportes and resgates sum the whole
 * recorded history (there is no period here), so the labels say "desde o
 * início"; the note explains the other two instead of hiding them in a
 * hover-only tooltip.
 */
export function InvestmentWorkspaceSummary({
  summary,
  operations,
}: {
  summary: InvestmentSummary | null;
  operations: InvestmentOperation[];
}) {
  const movements = accountMovements(operations);

  return (
    <SummaryStrip
      label="Valor atual"
      value={<Money value={summary?.total_value ?? "0"} tone="balance" size="hero" />}
      items={[
        { label: "Aportes desde o início", value: <Money value={movements.deposits} tone="neutral" /> },
        { label: "Resgates desde o início", value: <Money value={movements.withdrawals} tone="neutral" /> },
        { label: "Saldo para investir", value: <Money value={summary?.cash_balance ?? "0"} tone="neutral" /> },
        { label: "Informado pela instituição", value: <Money value={summary?.synced_value ?? "0"} tone="neutral" /> },
      ]}
      note="Saldo para investir: já saiu da conta corrente e ainda não virou posição. Informado pela instituição: parte do total que vem da integração."
    />
  );
}
