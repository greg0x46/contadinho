import { creditUsagePercent, creditUsageShare } from "../../presentation/accountLabels";

/** Renders how much of a card's limit is committed. Returns null when the
 *  ratio couldn't be computed (no limit, no balance, or a bank account), so
 *  callers can drop it in unconditionally. */
export function CreditUsageMeter({ ratio }: { ratio: string | null }) {
  if (ratio === null) return null;
  const share = creditUsageShare(ratio);

  return (
    <div className="debt-row-progress">
      <div className="debt-row-meter" aria-hidden="true">
        <span
          className="debt-row-meter-segment debt-row-meter-remaining"
          style={{ width: `${share}%` }}
        />
        <span
          className="debt-row-meter-segment debt-row-meter-paid"
          style={{ width: `${100 - share}%` }}
        />
      </div>
      <span className="debt-row-caption">{creditUsagePercent(ratio)} do limite usado</span>
    </div>
  );
}
