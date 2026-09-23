import type { Account, AccountClosingDaySource, AccountType } from "../api/contracts";
import { formatBRL, formatMoney } from "./money";

const accountTypeLabels: Record<AccountType, string> = {
  BANK: "Conta bancária",
  CREDIT: "Cartão de crédito",
};

export function accountTypeLabel(type: AccountType | null): string {
  if (type === null) return "Tipo não informado";
  return accountTypeLabels[type];
}

// Pluggy's account subtype is free text from the provider, not a closed enum
// contadinho-go validates, so this map only translates the values we've
// actually seen — anything else falls back to the raw string.
const accountSubtypeLabels: Record<string, string> = {
  CHECKING_ACCOUNT: "Conta corrente",
  SAVINGS_ACCOUNT: "Poupança",
  CREDIT_CARD: "Cartão de crédito",
};

export function accountSubtypeLabel(subtype: string | null): string | null {
  if (subtype === null) return null;
  return accountSubtypeLabels[subtype] ?? subtype;
}

export function closingDayLabel(day: number | null): string {
  return day === null ? "Não informado" : `Todo dia ${day}`;
}

const closingDaySourceHints: Record<AccountClosingDaySource, string> = {
  manual: "Definido por você",
  informado: "Informado pela instituição",
  estimado: "Estimado a partir da última fatura fechada",
};

export function closingDaySourceHint(source: AccountClosingDaySource | null): string {
  return source === null
    ? "A instituição não informa o fechamento deste cartão — defina o dia manualmente"
    : closingDaySourceHints[source];
}

export function maskedCardNumber(number: string): string {
  return `•••• ${number}`;
}

/** An account's own number is the full string the institution sends (often
 *  with a check digit or dashes); only the last four digits are meaningful
 *  as identity, so that's all this shows. */
export function maskedAccountNumber(number: string | null): string | null {
  if (number === null) return null;
  const digits = number.replace(/\D/g, "");
  if (digits === "") return null;
  return maskedCardNumber(digits.slice(-4));
}

/** Formats a 0–1 usage ratio as a pt-BR percentage. Values above 100% are
 *  real (the limit was exceeded) and are shown as-is. */
export function creditUsagePercent(ratio: string): string {
  return `${(Number(ratio) * 100).toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`;
}

/** The share of the limit to paint in the meter, clamped so an over-limit
 *  card still renders a full bar instead of overflowing it. */
export function creditUsageShare(ratio: string): number {
  return Math.min(100, Math.max(0, Number(ratio) * 100));
}

export function isCreditAccount(account: Account): boolean {
  return account.account_type === "CREDIT";
}

export function accountDisplayName(account: Account): string {
  return account.name ?? account.institution ?? "Conta sem nome";
}

/** Accounts carry their own currency, unlike the rest of the app, which is
 *  BRL by construction — so a non-BRL account keeps its own code instead of
 *  being mislabelled with a real. */
export function formatAccountMoney(value: string | null, currencyCode: string | null): string {
  if (value === null) return "—";
  if (currencyCode === null || currencyCode === "BRL") return formatBRL(value);
  return formatMoney(value, currencyCode);
}
