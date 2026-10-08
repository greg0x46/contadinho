import { requiredRecord } from "./shared";

export type TransactionsPeriodBasis = "occurred_at" | "paid_at";

export interface Preferences {
  transactions_period_basis: TransactionsPeriodBasis;
}

export function parsePreferences(value: unknown): Preferences {
  const preferences = requiredRecord(value, ["transactions_period_basis"], "Preferências inválidas.");
  if (preferences.transactions_period_basis !== "occurred_at" && preferences.transactions_period_basis !== "paid_at") {
    throw new TypeError("Preferências inválidas.");
  }
  return { transactions_period_basis: preferences.transactions_period_basis };
}
