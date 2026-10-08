export interface CurrencyTotals {
  currency_code: string;
  inflow: string;
  outflow: string;
  balance: string;
}

const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

export const isValidDate = (value: unknown): value is string =>
  typeof value === "string" && value.trim() !== "" && !Number.isNaN(Date.parse(value));

export const isNullableString = (value: unknown): value is string | null =>
  value === null || typeof value === "string";

export const isCount = (value: unknown): value is number =>
  Number.isInteger(value) && typeof value === "number" && value >= 0;

export const isUuid = (value: string): boolean => uuidPattern.test(value);

const decimalPattern = /^-?(0|[1-9][0-9]*)(\.[0-9]+)?$/;

export const dateOnlyPattern = /^\d{4}-\d{2}-\d{2}$/;

function hasOnlyKeys(value: Record<string, unknown>, keys: readonly string[]): boolean {
  const actual = Object.keys(value);
  return actual.length === keys.length && actual.every((key) => keys.includes(key));
}

export function requiredRecord(
  value: unknown,
  keys: readonly string[],
  message = "Resposta de transações inválida.",
): Record<string, unknown> {
  if (!isRecord(value) || !hasOnlyKeys(value, keys)) throw new TypeError(message);
  return value;
}

/**
 * A few response fields were added after the first released frontend. Keep
 * their absence readable for cached responses and test fixtures, while still
 * rejecting misspelled server fields instead of quietly rendering nonsense.
 */
export function requiredRecordWithOptionalKeys(
  value: unknown,
  requiredKeys: readonly string[],
  optionalKeys: readonly string[],
  message = "Resposta de transações inválida.",
): Record<string, unknown> {
  if (!isRecord(value)) throw new TypeError(message);
  const allowed = [...requiredKeys, ...optionalKeys];
  const actual = Object.keys(value);
  if (!requiredKeys.every((key) => key in value) || !actual.every((key) => allowed.includes(key))) {
    throw new TypeError(message);
  }
  return value;
}

export function decimal(value: unknown): string {
  if (typeof value !== "string" || !decimalPattern.test(value)) {
    throw new TypeError("Valor monetário inválido.");
  }
  return value;
}

export function nullableDecimal(value: unknown): string | null {
  return value === null ? null : decimal(value);
}

export function nullableText(value: unknown): string | null {
  if (!isNullableString(value)) throw new TypeError("Texto opcional inválido.");
  return value;
}

export function nullableDate(value: unknown): string | null {
  if (!(value === null || isValidDate(value))) throw new TypeError("Data opcional inválida.");
  return value;
}

export function positiveCount(value: unknown): number {
  if (!isCount(value) || value < 1) throw new TypeError("Contagem inválida.");
  return value;
}

export function parseCurrencyTotals(value: unknown): CurrencyTotals {
  const item = requiredRecord(value, ["currency_code", "inflow", "outflow", "balance"]);
  if (typeof item.currency_code !== "string" || item.currency_code === "") {
    throw new TypeError("Moeda inválida.");
  }
  return {
    currency_code: item.currency_code,
    inflow: decimal(item.inflow),
    outflow: decimal(item.outflow),
    balance: decimal(item.balance),
  };
}

export function isNullableUuid(value: unknown): value is string | null {
  return value === null || (typeof value === "string" && isUuid(value));
}
