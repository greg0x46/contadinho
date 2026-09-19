import type { InvestmentOperation, InvestmentPosition } from "../../api/contracts";
import { subtractBRL, sumBRL } from "../../presentation/money";

/**
 * Money stays in decimal strings everywhere in the workspace. money.ts already
 * adds and subtracts BRL; the only extra operation the views need is
 * quantidade × custo médio, whose operands carry more than two decimals.
 * Doing it in BigInt keeps a position's acquisition cost exact instead of
 * letting a float decide whether a rentabilidade is a few cents off.
 */
function scaled(value: string): { digits: bigint; scale: number } {
  const negative = value.startsWith("-");
  const unsigned = negative ? value.slice(1) : value;
  const [integer = "0", fraction = ""] = unsigned.split(".");
  const digits = BigInt(`${integer === "" ? "0" : integer}${fraction}`);
  return { digits: negative ? -digits : digits, scale: fraction.length };
}

export function multiplyToBRL(left: string, right: string): string {
  const a = scaled(left);
  const b = scaled(right);
  const product = a.digits * b.digits;
  const scale = a.scale + b.scale;
  const negative = product < 0n;
  let magnitude = negative ? -product : product;
  if (scale < 2) {
    magnitude *= 10n ** BigInt(2 - scale);
  } else if (scale > 2) {
    const divisor = 10n ** BigInt(scale - 2);
    const remainder = magnitude % divisor;
    magnitude /= divisor;
    if (remainder * 2n >= divisor) magnitude += 1n;
  }
  const digits = magnitude.toString().padStart(3, "0");
  return `${negative ? "-" : ""}${digits.slice(0, -2)}.${digits.slice(-2)}`;
}

export function isZeroBRL(value: string | null): boolean {
  return value === null || /^-?0*(\.0*)?$/.test(value);
}

export type PositionYield =
  | { known: true; value: string }
  | { known: false; reason: string };

/**
 * Rentabilidade needs both a market value and an acquisition cost. When the
 * current value *is* the cost (no valuation ever recorded) the gain is not
 * zero, it is unknown — saying so is the whole point of valuation_basis.
 */
export function positionYield(position: InvestmentPosition): PositionYield {
  if (position.valuation_basis === "cost_basis") {
    return { known: false, reason: "Sem cotação registrada: o valor exibido é o custo acumulado." };
  }
  if (position.average_cost === null || isZeroBRL(position.average_cost)) {
    return { known: false, reason: "Sem custo de aquisição registrado." };
  }
  if (isZeroBRL(position.quantity)) {
    return { known: false, reason: "Sem quantidade registrada para comparar com o custo." };
  }
  return { known: true, value: subtractBRL(position.current_value, multiplyToBRL(position.average_cost, position.quantity)) };
}

export type MovementTotals = { deposits: string; withdrawals: string };

function totalOf(operations: InvestmentOperation[], kinds: InvestmentOperation["kind"][]): string {
  return sumBRL(operations.filter((operation) => kinds.includes(operation.kind)).map((operation) => operation.amount));
}

/** Cash entering and leaving the custody account — the patrimony transfers. */
export function accountMovements(operations: InvestmentOperation[]): MovementTotals {
  return {
    deposits: totalOf(operations, ["deposit"]),
    withdrawals: totalOf(operations, ["withdrawal"]),
  };
}

/**
 * A goal holds no cash, so its aportes are what was put into its positions
 * (compras e saldo inicial) and its resgates are what was sold out of them.
 */
export function goalMovements(operations: InvestmentOperation[]): MovementTotals {
  return {
    deposits: totalOf(operations, ["buy", "initial_balance"]),
    withdrawals: totalOf(operations, ["sell"]),
  };
}

/** The most recent date among the values that make a card's figures. */
export function latestDate(values: (string | null)[]): string | null {
  return values
    .filter((value): value is string => value !== null && value !== "")
    .map((value) => value.slice(0, 10))
    .sort()
    .at(-1) ?? null;
}

export function formatDate(value: string | null): string {
  if (value === null) return "Sem registro";
  const [year, month, day] = value.split("-");
  return year && month && day ? `${day}/${month}/${year}` : value;
}
