/**
 * The investment drafts keep amounts as decimal strings (that is what the API
 * takes), while `MoneyInput` is a number field with two decimals. These two
 * hops are the only place a draft amount touches a JS number, and only ever
 * a value the field has already rounded to cents.
 */
export function toMoneyInput(value: string | null): number | null {
  if (value === null || value === "") return null;
  const parsed = Number(value);
  return Number.isNaN(parsed) ? null : parsed;
}

export function fromMoneyInput(value: number | null): string | null {
  return value === null ? null : value.toFixed(2);
}
