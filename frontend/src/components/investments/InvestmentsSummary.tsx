import type { Investment } from "../../api/contracts";
import { investmentYield } from "../../presentation/investmentLabels";
import { sumBRL } from "../../presentation/money";
import { SummaryStrip } from "../layout";
import { Money } from "../shared/Money";

/** What the synced investments add up to: the amount applied and the yield the institution lets us state. */
export function InvestmentsSummary({ investments }: { investments: Investment[] }) {
  const balanceTotal = sumBRL(investments.map((investment) => investment.balance ?? "0"));
  const yields = investments.map((investment) => investmentYield(investment)).filter((y) => y !== null);
  const yieldTotal = sumBRL(yields.map((y) => y.value));
  const missing = investments.length - yields.length;

  return (
    <SummaryStrip
      label="Aplicado nos sincronizados"
      value={<Money value={balanceTotal} tone="balance" size="hero" />}
      items={[
        { label: "Rendimento acumulado", value: <Money value={yieldTotal} tone="result" /> },
        ...(missing > 0 ? [{ label: "Sem dado de rendimento", value: String(missing) }] : []),
      ]}
    />
  );
}
