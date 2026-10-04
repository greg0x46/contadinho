/**
 * The thin progress meter of a payable: the settled share in the brand green
 * on a neutral track. There is deliberately no "remaining" segment — the
 * unpaid part is just the empty track, never a red bar (least of all for a
 * receivable, where "remaining" is money someone owes you).
 */
export function PayableMeter({ percent, className }: { percent: number; className?: string }) {
  return (
    <span className={["payable-meter", className].filter(Boolean).join(" ")} aria-hidden="true">
      <span className="payable-meter-fill" style={{ width: `${percent}%` }} />
    </span>
  );
}
