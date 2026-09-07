const DAY = 86_400_000;

function parseDate(value: unknown): Date | null {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const date = new Date(`${value}T00:00:00Z`);
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value
    ? date : null;
}

export function periodNavigation(from: unknown, to: unknown) {
  const start = parseDate(from);
  const end = parseDate(to);
  if (!start || !end || start > end) return null;
  const afterEnd = new Date(end.getTime() + DAY);
  const wholeMonths = start.getUTCDate() === 1 && afterEnd.getUTCDate() === 1;
  const months = (afterEnd.getUTCFullYear() - start.getUTCFullYear()) * 12
    + afterEnd.getUTCMonth() - start.getUTCMonth();
  const isYear = wholeMonths && months === 12 && start.getUTCMonth() === 0;
  const unit = isYear ? "ano" : wholeMonths && months === 1 ? "mês" : "período";
  const format = (date: Date) => date.toLocaleDateString("pt-BR", { timeZone: "UTC" });
  const label = isYear ? String(start.getUTCFullYear())
    : wholeMonths && months === 1
      ? start.toLocaleDateString("pt-BR", { month: "long", year: "numeric", timeZone: "UTC" })
      : `${format(start)} – ${format(end)}`;

  const shift = (direction: -1 | 1): [string, string] | null => {
    let nextStart: Date;
    let nextEnd: Date;
    if (wholeMonths) {
      nextStart = new Date(start);
      nextStart.setUTCMonth(start.getUTCMonth() + months * direction);
      nextEnd = new Date(nextStart);
      nextEnd.setUTCMonth(nextStart.getUTCMonth() + months);
      nextEnd.setUTCDate(0);
    } else {
      const offset = (end.getTime() - start.getTime() + DAY) * direction;
      nextStart = new Date(start.getTime() + offset);
      nextEnd = new Date(end.getTime() + offset);
    }
    if (nextStart.getUTCFullYear() < 1 || nextEnd.getUTCFullYear() > 9999) return null;
    return [nextStart.toISOString().slice(0, 10), nextEnd.toISOString().slice(0, 10)];
  };
  return { label, previousLabel: `${unit[0].toUpperCase()}${unit.slice(1)} anterior`,
    nextLabel: `Próximo ${unit}`, previous: shift(-1), next: shift(1) };
}
