const formatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "short",
  timeStyle: "medium",
});

export function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Data inválida" : formatter.format(date);
}

export function formatOptionalDate(value: string | null): string {
  return value === null ? "Ainda não disponível" : formatDate(value);
}

// Bill and card-limit dates are calendar days anchored at midnight UTC, so
// rendering them with formatDate would tack on a meaningless "00:00:00" — and
// reading them in the local timezone would slide them a day backwards.
const dayFormatter = new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeZone: "UTC" });

export function formatDay(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Data inválida" : dayFormatter.format(date);
}

export function formatOptionalDay(value: string | null): string {
  return value === null ? "—" : formatDay(value);
}

// An instant — a transaction's occurred_at, a card's last use — is a point in
// time, not a calendar day, and belongs in the reader's own timezone: that's
// how the transactions screen renders it, and the two screens show the same
// rows. Pinning it to UTC like formatDay would push anything after 21:00 in
// Brazil onto the next day, so the same purchase would carry two dates.
const localDayFormatter = new Intl.DateTimeFormat("pt-BR", { dateStyle: "short" });

export function formatLocalDay(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Data inválida" : localDayFormatter.format(date);
}

export function formatOptionalLocalDay(value: string | null): string {
  return value === null ? "—" : formatLocalDay(value);
}
