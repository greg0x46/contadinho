import { formatMoney } from "../../presentation/money";
import type { MoneyToneKind } from "../../presentation/money";
import { Money } from "../shared/Money";

/**
 * An account's figure under the app's money rule. Accounts are the one place
 * a non-BRL amount can appear (see `formatAccountMoney`), and `Money` is BRL
 * by construction, so a foreign-currency account keeps its own code in plain
 * tabular text instead of being mislabelled with a real.
 */
export function AccountMoney({
  value,
  currencyCode,
  tone,
  size,
}: {
  value: string | null;
  currencyCode: string | null;
  tone: MoneyToneKind;
  size?: "row" | "hero";
}) {
  if (value === null) return <span className="money">—</span>;
  if (currencyCode === null || currencyCode === "BRL") return <Money value={value} tone={tone} size={size} />;
  return <span className={`money${size ? ` money-size-${size}` : ""}`}>{formatMoney(value, currencyCode)}</span>;
}
