// Exact arithmetic for investment quantities, prices and reconciliation parcels.
// Formatting is separate: rounding a funding amount before recording its trade
// can leave a fractional purchase without enough cash.
function scaled(value: string): { digits: bigint; scale: number } {
  const negative = value.startsWith("-");
  const [whole = "0", fraction = ""] = (negative ? value.slice(1) : value).split(".");
  const digits = BigInt(`${whole || "0"}${fraction}`);
  return { digits: negative ? -digits : digits, scale: fraction.length };
}
function text(digits: bigint, scale: number): string {
  if (scale < 2) { digits *= 10n ** BigInt(2 - scale); scale = 2; }
  const negative = digits < 0n;
  const raw = (negative ? -digits : digits).toString().padStart(scale + 1, "0");
  return `${negative ? "-" : ""}${raw.slice(0, -scale)}.${raw.slice(-scale)}`;
}
export function sumDecimals(values: string[]): string {
  const parts = values.map(scaled);
  const scale = Math.max(2, ...parts.map((part) => part.scale));
  return text(parts.reduce((total, part) => total + part.digits * 10n ** BigInt(scale - part.scale), 0n), scale);
}
export function subtractDecimals(left: string, right: string): string {
  return sumDecimals([left, right.startsWith("-") ? right.slice(1) : `-${right}`]);
}
export function multiplyDecimals(left: string, right: string): string {
  const a = scaled(left), b = scaled(right);
  return text(a.digits * b.digits, a.scale + b.scale);
}
export function isPositiveDecimal(value: string): boolean { return scaled(value).digits > 0n; }
export function isZeroDecimal(value: string): boolean { return scaled(value).digits === 0n; }
