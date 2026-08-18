export function formatMoney(value: string, currencyCode: string): string {
  const negative = value.startsWith("-");
  const unsigned = negative ? value.slice(1) : value;
  const [integer, fraction] = unsigned.split(".");
  const grouped = (integer ?? "0").replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const formatted = fraction === undefined ? grouped : `${grouped},${fraction}`;
  return `${negative ? "-" : ""}${currencyCode}\u00a0${formatted}`;
}

function fixedTwo(value: string): string {
  const negative = value.startsWith("-");
  const unsigned = negative ? value.slice(1) : value;
  const [integer = "0", fraction = ""] = unsigned.split(".");
  const padded = fraction.padEnd(3, "0");
  let cents = BigInt(`${integer}${padded.slice(0, 2)}`);
  if (padded[2] >= "5") cents += 1n;
  const digits = cents.toString().padStart(3, "0");
  const whole = digits.slice(0, -2).replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const decimals = digits.slice(-2);
  return `${negative ? "-" : ""}${whole},${decimals}`;
}

export function formatBRL(value: string): string {
  return `${value.startsWith("-") ? "-" : ""}R$\u00a0${fixedTwo(value).replace("-", "")}`;
}

export function formatSignedBRL(
  value: string,
  classification: "inflow" | "outflow" | "unclassified",
): string {
  const unsigned = value.startsWith("-") ? value.slice(1) : value;
  const sign = classification === "inflow" ? "+" : classification === "outflow" ? "-" : "";
  return `${sign}R$\u00a0${fixedTwo(unsigned)}`;
}

export const moneySourceLabel = (source: "account_currency" | "transaction_currency") =>
  source === "account_currency" ? "Valor na moeda da conta" : "Valor original";

// toCents/fromCents let callers sum a handful of BRL decimal strings
// (e.g. accumulating several MonthSummary rows client-side) without a
// bignum dependency — same BigInt-cents technique fixedTwo already uses,
// just exposed for addition instead of only formatting.
function toCents(value: string): bigint {
  const negative = value.startsWith("-");
  const unsigned = negative ? value.slice(1) : value;
  const [integer = "0", fraction = ""] = unsigned.split(".");
  const cents = BigInt(`${integer}${fraction.padEnd(2, "0").slice(0, 2)}`);
  return negative ? -cents : cents;
}

function fromCents(cents: bigint): string {
  const negative = cents < 0n;
  const digits = (negative ? -cents : cents).toString().padStart(3, "0");
  const whole = digits.slice(0, -2);
  const decimals = digits.slice(-2);
  return `${negative ? "-" : ""}${whole}.${decimals}`;
}

export function sumBRL(values: string[]): string {
  return fromCents(values.reduce((total, value) => total + toCents(value), 0n));
}
