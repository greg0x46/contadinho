import type { Account, AccountClosingDaySource, AccountType } from "../api/contracts";
import { formatOptionalDayMonth } from "./dates";
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

/** Compact form for a row caption ("Fecha dia 2"), as opposed to
 *  closingDayLabel's "Todo dia 2" used in the account's own detail page. */
export function closingDayShortLabel(day: number | null): string | null {
  return day === null ? null : `Fecha dia ${day}`;
}

export function maskedCardNumber(number: string): string {
  return `•••• ${number}`;
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

/** Secondary, low-weight facts about an account row: the real institution
 *  (Pluggy's connector name, e.g. "Itaú" — omitted when it's already the
 *  displayed name, which happens whenever the account has no name of its
 *  own), its subtype, and its masked number. */
export function accountMetaParts(account: Account): string[] {
  const parts: string[] = [];
  if (account.institution !== null && account.institution !== accountDisplayName(account)) {
    parts.push(account.institution);
  }
  const subtype = accountSubtypeLabel(account.account_subtype);
  if (subtype !== null) parts.push(subtype);
  if (account.number !== null) parts.push(maskedCardNumber(account.number));
  return parts;
}

/** The credit card row's schedule caption: "Vence DD/MM · Fecha dia D",
 *  each half dropped when the underlying date isn't known. */
export function creditCardScheduleParts(account: Account): string[] {
  const parts: string[] = [];
  if (account.balance_due_date !== null) {
    parts.push(`Vence ${formatOptionalDayMonth(account.balance_due_date)}`);
  }
  const closing = closingDayShortLabel(account.closing_day);
  if (closing !== null) parts.push(closing);
  return parts;
}

/** Accounts carry their own currency, unlike the rest of the app, which is
 *  BRL by construction — so a non-BRL account keeps its own code instead of
 *  being mislabelled with a real. */
export function formatAccountMoney(value: string | null, currencyCode: string | null): string {
  if (value === null) return "—";
  if (currencyCode === null || currencyCode === "BRL") return formatBRL(value);
  return formatMoney(value, currencyCode);
}
