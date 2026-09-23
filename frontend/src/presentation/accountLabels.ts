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
  return account.name ?? account.institution_name ?? "Conta sem nome";
}

// A registered institution name often carries a corporate suffix and a
// trailing descriptor ("Nu Pagamentos S.A. - Instituição de Pagamento") that
// nobody needs to recognize the bank — the part before the first " - " is
// already the brand, and the suffix on it is noise.
const institutionSuffixPattern = /\s+(s\.?\s?a\.?|s\/a|ltda\.?|eireli\.?)$/i;

export function shortInstitutionName(name: string): string {
  const brand = name.split(" - ")[0].trim();
  const short = brand.replace(institutionSuffixPattern, "").trim();
  return short || brand;
}

/** The account detail page's heading: the institution first, since that's
 *  how a user recognizes the account, falling back to whatever name the
 *  account itself carries. */
export function accountHeaderTitle(account: Account): string {
  if (account.institution_name !== null) return shortInstitutionName(account.institution_name);
  return account.name ?? "Conta sem nome";
}

/** "Conta corrente · •••• 0966" — the compact identity line under a name,
 *  shared by the account list rows and the detail page header. */
export function accountIdentityLine(account: Account): string {
  const parts = [accountSubtypeLabel(account.account_subtype), maskedAccountNumber(account.number)];
  return parts.filter((part): part is string => part !== null).join(" · ");
}

/** Accounts carry their own currency, unlike the rest of the app, which is
 *  BRL by construction — so a non-BRL account keeps its own code instead of
 *  being mislabelled with a real. */
export function formatAccountMoney(value: string | null, currencyCode: string | null): string {
  if (value === null) return "—";
  if (currencyCode === null || currencyCode === "BRL") return formatBRL(value);
  return formatMoney(value, currencyCode);
}
