import { scenarioKinds } from "./scenarios";
import type { ScenarioKind } from "./scenarios";
import { dateOnlyPattern, decimal, isNullableString, isNullableUuid, isUuid, isValidDate, nullableDecimal, requiredRecord } from "./shared";

export const projectionTiers = ["realizado", "confirmado", "projetado", "hipotetico"] as const;

export type ProjectionTier = (typeof projectionTiers)[number];

// Planned transactions use the same serialized source vocabulary as the
// Timeline endpoint. Keeping both parsers on one set of values prevents a
// recurring/plan event returned by /planned-transactions from being rejected
// while the Timeline accepts it.
export const projectionSources = ["real", "recorrente", "plano_pagamento", "cenario"] as const;

export type ProjectionSource = (typeof projectionSources)[number];

export interface PlannedTransaction {
  scenario_id: string;
  event_key: string;
  scenario_kind: ScenarioKind;
  date: string;
  description: string;
  amount: string;
  category_id: string | null;
  category_name: string;
  tier: ProjectionTier;
  source: ProjectionSource;
  payable_id: string | null;
  realized: boolean;
  realization_origin: string;
  detached: boolean;
}

const plannedTransactionKeys = [
  "scenario_id",
  "event_key",
  "scenario_kind",
  "date",
  "description",
  "amount",
  "category_id",
  "category_name",
  "tier",
  "source",
  "payable_id",
  "realized",
  "realization_origin",
  "detached",
] as const;

export function parsePlannedTransaction(value: unknown): PlannedTransaction {
  const item = requiredRecord(value, plannedTransactionKeys, "Evento previsto inválido.");
  if (
    typeof item.scenario_id !== "string" ||
    !isUuid(item.scenario_id) ||
    typeof item.event_key !== "string" ||
    item.event_key === "" ||
    !scenarioKinds.includes(item.scenario_kind as ScenarioKind) ||
    typeof item.date !== "string" ||
    !dateOnlyPattern.test(item.date) ||
    typeof item.description !== "string" ||
    item.description === "" ||
    !isNullableUuid(item.category_id) ||
    typeof item.category_name !== "string" ||
    !projectionTiers.includes(item.tier as ProjectionTier) ||
    !projectionSources.includes(item.source as ProjectionSource) ||
    !isNullableUuid(item.payable_id) ||
    typeof item.realized !== "boolean" ||
    typeof item.realization_origin !== "string" ||
    typeof item.detached !== "boolean"
  ) {
    throw new TypeError("Evento previsto inválido.");
  }
  return {
    scenario_id: item.scenario_id,
    event_key: item.event_key,
    scenario_kind: item.scenario_kind as ScenarioKind,
    date: item.date,
    description: item.description,
    amount: decimal(item.amount),
    category_id: item.category_id as string | null,
    category_name: item.category_name,
    tier: item.tier as ProjectionTier,
    source: item.source as ProjectionSource,
    payable_id: item.payable_id as string | null,
    realized: item.realized,
    realization_origin: item.realization_origin,
    detached: item.detached,
  };
}

export function parsePlannedTransactionList(value: unknown): PlannedTransaction[] {
  if (!Array.isArray(value)) throw new TypeError("Lista de eventos previstos inválida.");
  return value.map(parsePlannedTransaction);
}

export interface ScenarioRealization {
  id: string;
  scenario_id: string;
  scenario_transaction_id: string | null;
  occurrence_date: string | null;
  transaction_id: string | null;
  relation_type: "settlement" | "allocation" | "reconciliation";
  state: "linked" | "detached";
  origin: string;
  allocated_amount: string | null;
  linked_amount: string | null;
  created_at: string;
}

const scenarioRealizationKeys = [
  "id",
  "scenario_id",
  "scenario_transaction_id",
  "occurrence_date",
  "transaction_id",
  "relation_type",
  "state",
  "origin",
  "allocated_amount",
  "linked_amount",
  "created_at",
] as const;

export function parseScenarioRealization(value: unknown): ScenarioRealization {
  const item = requiredRecord(value, scenarioRealizationKeys, "Realização inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.scenario_id !== "string" ||
    !isUuid(item.scenario_id) ||
    !isNullableUuid(item.scenario_transaction_id) ||
    !(item.occurrence_date === null || (typeof item.occurrence_date === "string" && dateOnlyPattern.test(item.occurrence_date))) ||
    !isNullableUuid(item.transaction_id) ||
    !["settlement", "allocation", "reconciliation"].includes(item.relation_type as string) ||
    !["linked", "detached"].includes(item.state as string) ||
    typeof item.origin !== "string" ||
    !isNullableString(item.allocated_amount) ||
    !isNullableString(item.linked_amount) ||
    !isValidDate(item.created_at)
  ) {
    throw new TypeError("Realização inválida.");
  }
  return {
    id: item.id,
    scenario_id: item.scenario_id,
    scenario_transaction_id: item.scenario_transaction_id as string | null,
    occurrence_date: item.occurrence_date as string | null,
    transaction_id: item.transaction_id as string | null,
    relation_type: item.relation_type as ScenarioRealization["relation_type"],
    state: item.state as ScenarioRealization["state"],
    origin: item.origin,
    allocated_amount: nullableDecimal(item.allocated_amount),
    linked_amount: nullableDecimal(item.linked_amount),
    created_at: item.created_at,
  };
}
