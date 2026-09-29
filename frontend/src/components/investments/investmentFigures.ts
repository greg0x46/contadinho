import type { Investment, InvestmentOperation, InvestmentPosition } from "../../api/contracts";
import { yieldUnavailable } from "../../presentation/investmentLabels";
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

/** Provider holdings by id, so a synced position can read its linked investment's figures. */
export type LinkedInvestments = ReadonlyMap<string, Investment>;

export type PositionYield =
  | { known: true; value: string; percent: number | null; basis: string; gross: string }
  | { known: false; reason: string };

function linkedInvestment(position: InvestmentPosition, linked: LinkedInvestments): Investment | undefined {
  return position.linked_investment_id === null ? undefined : linked.get(position.linked_investment_id);
}

/**
 * A synced position has no local cost basis — the provider does not send a
 * trustworthy one — so its rendimento is the one the backend already states
 * for the linked investment (Pluggy's amount_profit, or balance minus net
 * contributed). The % is over the principal: amountOriginal when the
 * institution sends it, otherwise value minus that same gain.
 */
function syncedYield(position: InvestmentPosition, linked: LinkedInvestments, cost: string): PositionYield {
  const investment = linkedInvestment(position, linked);
  if (investment === undefined) {
    return { known: false, reason: "Rendimento informado pela instituição ainda não carregado." };
  }
  if (investment.yield_value === null) {
    return { known: false, reason: yieldUnavailable(investment).hint || "A instituição não informou o rendimento." };
  }
  const gain = sumBRL([investment.yield_value]);
  const basis =
    investment.amount_original !== null && !isZeroBRL(investment.amount_original)
      ? sumBRL([investment.amount_original])
      : subtractBRL(position.current_value, gain);
  const principal = Number(basis);
  const net = subtractBRL(gain, cost);
  return { known: true, value: net, percent: principal > 0 ? (Number(net) / principal) * 100 : null, basis, gross: gain };
}

/**
 * Rentabilidade needs both a market value and an acquisition cost. When the
 * current value *is* the cost (no valuation ever recorded) the gain is not
 * zero, it is unknown — saying so is the whole point of valuation_basis.
 *
 * The figure is líquido: the gross gain (market value over acquisition cost,
 * or the provider's own profit) minus `cost`, the position's taxas e impostos
 * as costsByPosition states them (fees/taxes of its movimentações plus the
 * IR/IOF the institution provisioned on a synced holding). `gross` keeps the
 * figure before that deduction for tooltips.
 */
export function positionYield(position: InvestmentPosition, linked: LinkedInvestments, cost = "0"): PositionYield {
  if (position.source === "synced") return syncedYield(position, linked, cost);
  if (position.valuation_basis === "cost_basis") {
    return { known: false, reason: "Sem cotação registrada: o valor exibido é o custo acumulado." };
  }
  if (position.average_cost === null || isZeroBRL(position.average_cost)) {
    return { known: false, reason: "Sem custo de aquisição registrado." };
  }
  if (isZeroBRL(position.quantity)) {
    return { known: false, reason: "Sem quantidade registrada para comparar com o custo." };
  }
  const basis = multiplyToBRL(position.average_cost, position.quantity);
  const gross = subtractBRL(position.current_value, basis);
  const net = subtractBRL(gross, cost);
  return { known: true, value: net, percent: (Number(net) / Number(basis)) * 100, basis, gross };
}

export type YieldAggregate = { gain: string; percent: number | null };

/**
 * The same R$-gain-over-principal math as positionYield, summed across a
 * group of positions (an account, a goal, the whole workspace). Positions
 * with unknown rentabilidade — or in a foreign currency — sit out of both
 * sides of the ratio rather than being counted as zero gain.
 */
export function aggregateYield(
  positions: InvestmentPosition[],
  linked: LinkedInvestments,
  costs: Record<string, string> = {},
): YieldAggregate {
  const gains: string[] = [];
  const bases: string[] = [];
  for (const position of positions) {
    if (position.currency_code !== "BRL") continue;
    const estimate = positionYield(position, linked, costs[position.id] ?? "0");
    if (!estimate.known) continue;
    gains.push(estimate.value);
    bases.push(estimate.basis);
  }
  const gain = sumBRL(gains);
  const basis = Number(sumBRL(bases));
  return { gain, percent: basis > 0 ? (Number(gain) / basis) * 100 : null };
}

export function formatPercent(value: number): string {
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toLocaleString("pt-BR", { maximumFractionDigits: 1, minimumFractionDigits: 1 })}%`;
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

const present = (value: string | null): value is string => value !== null;

/** IR and IOF the institution reports as provisioned on a synced holding. */
function syncedTaxes(position: InvestmentPosition, linked: LinkedInvestments): string[] {
  if (position.source !== "synced") return [];
  const investment = linkedInvestment(position, linked);
  return investment === undefined ? [] : [investment.taxes, investment.taxes2].filter(present);
}

/**
 * Custo of a group: taxas e impostos lançados nas movimentações, plus the IR
 * and IOF the institution reports on its synced holdings.
 */
export function groupCost(
  operations: InvestmentOperation[],
  positions: InvestmentPosition[],
  linked: LinkedInvestments,
): string {
  return sumBRL([
    ...operations.flatMap((operation) => [operation.fees, operation.taxes].filter(present)),
    ...positions.flatMap((position) => syncedTaxes(position, linked)),
  ]);
}

/** Same as groupCost, per position, so each row of the positions list shows its own custo. */
export function costsByPosition(
  operations: InvestmentOperation[],
  positions: InvestmentPosition[],
  linked: LinkedInvestments,
): Record<string, string> {
  const byPosition = new Map<string, string[]>();
  const push = (positionId: string, values: string[]) =>
    byPosition.set(positionId, [...(byPosition.get(positionId) ?? []), ...values]);
  for (const operation of operations) {
    if (operation.position_id !== null) push(operation.position_id, [operation.fees, operation.taxes].filter(present));
  }
  for (const position of positions) push(position.id, syncedTaxes(position, linked));
  const result: Record<string, string> = {};
  for (const [positionId, values] of byPosition) result[positionId] = sumBRL(values);
  return result;
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

export function positionCountLabel(count: number): string {
  return count === 1 ? "1 posição" : `${count} posições`;
}

/** A decimal quantity in pt-BR notation, without the trailing zeros the API pads it with. */
export function formatQuantity(value: string): string {
  const negative = value.startsWith("-");
  const unsigned = negative ? value.slice(1) : value;
  const [integer = "0", fraction = ""] = unsigned.split(".");
  const grouped = (integer === "" ? "0" : integer).replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const trimmed = fraction.replace(/0+$/, "");
  return `${negative ? "-" : ""}${grouped}${trimmed ? `,${trimmed}` : ""}`;
}

export function formatDate(value: string | null): string {
  if (value === null) return "Sem registro";
  const [year, month, day] = value.split("-");
  return year && month && day ? `${day}/${month}/${year}` : value;
}
