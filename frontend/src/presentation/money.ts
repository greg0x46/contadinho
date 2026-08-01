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
