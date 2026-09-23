import { creditUsagePercent, creditUsageShare, formatAccountMoney } from "../../presentation/accountLabels";

function usageLevel(share: number): "normal" | "warning" | "over" {
  if (share >= 100) return "over";
  if (share >= 80) return "warning";
  return "normal";
}

/**
 * How much of a card's limit is committed: a single bar, plain by default —
 * it only turns into an alert color once usage is actually close to (80%+)
 * or past (100%+) the limit, never a paid/remaining split. Returns null
 * when there's no ratio to show (no limit, no balance, or a bank account),
 * so callers can drop it in unconditionally.
 */
export function LimitUsage({
  ratio,
  available = null,
  currencyCode = null,
}: {
  ratio: string | null;
  /** Appended to the caption as "· R$ x disponível" when given. */
  available?: string | null;
  currencyCode?: string | null;
}) {
  if (ratio === null) return null;
  const share = creditUsageShare(ratio);
  const percent = creditUsagePercent(ratio);

  return (
    <div className={`limit-usage is-${usageLevel(share)}`}>
      <span className="limit-usage-caption">
        {percent} do limite utilizado
        {available !== null && ` · ${formatAccountMoney(available, currencyCode)} disponível`}
      </span>
      <div
        className="limit-usage-track"
        role="progressbar"
        aria-valuenow={Math.round(share)}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={`${percent} do limite utilizado`}
      >
        <div className="limit-usage-fill" style={{ width: `${share}%` }} />
      </div>
    </div>
  );
}
