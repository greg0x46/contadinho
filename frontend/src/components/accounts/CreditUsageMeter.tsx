import { creditUsagePercent, creditUsageLevel, creditUsageShare } from "../../presentation/accountLabels";

/** Renders how much of a card's limit is committed: one fill bar, not the
 *  red/paid-vs-green/remaining pairing that fits a debt payoff (see
 *  payables.css) but not spending room. Color only escalates once usage
 *  actually needs attention — see creditUsageLevel. Returns null when the
 *  ratio couldn't be computed (no limit, no balance, or a bank account), so
 *  callers can drop it in unconditionally. */
export function CreditUsageMeter({ ratio }: { ratio: string | null }) {
  if (ratio === null) return null;
  const share = creditUsageShare(ratio);
  const level = creditUsageLevel(ratio);

  return (
    <div className="credit-usage-meter">
      <div className="credit-usage-meter-track" aria-hidden="true">
        <span className={`credit-usage-meter-fill is-${level}`} style={{ width: `${share}%` }} />
      </div>
      <span className="credit-usage-meter-caption">{creditUsagePercent(ratio)} do limite utilizado</span>
    </div>
  );
}
