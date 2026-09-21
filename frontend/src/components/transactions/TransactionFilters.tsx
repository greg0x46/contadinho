import { useMemo, type ReactNode } from "react";

import type {
  TransactionFilters as Filters,
  TransactionQueryResult,
} from "../../api/contracts";
import {
  ConfigurableFilters,
  type FilterConfig,
} from "../filters/ConfigurableFilters";
import { categoryFilterOptions, renderCategoryIcon } from "../../presentation/categoryLabels";
import { classificationLabel } from "../../presentation/transactionStatus";

/**
 * The collection controls of the transactions list. The period is not one
 * of them: it is the page's context and lives in the Page header, so the
 * values here only carry `date_from`/`date_to` through untouched.
 */
export function TransactionFilters({
  applied,
  emptyValues,
  facets,
  onApply,
  onClear,
  end,
}: {
  applied: Filters;
  emptyValues?: Filters;
  facets: TransactionQueryResult["available_filters"] | undefined;
  onApply: (filters: Filters) => void;
  onClear: () => void;
  end?: ReactNode;
}) {
  const config = useMemo<FilterConfig<Filters>[]>(
    () => [
      {
        key: "description",
        label: "Descrição",
        type: "text",
        placement: "main",
        placeholder: "Buscar transações…",
        debounceMs: 300,
        formatActive: (value) => `Busca: ${String(value)}`,
      },
      {
        key: "card_balance",
        label: "Somente lançamentos do balanço do cartão",
        type: "boolean",
        placement: "advanced",
        formatActive: () => "Balanço do cartão",
      },
      {
        key: "credit_card",
        label: "Cartão de crédito",
        type: "boolean",
        placement: "advanced",
        formatActive: () => "Cartão de crédito",
      },
      {
        key: "account_id",
        label: "Conta",
        type: "select",
        placement: "advanced",
        placeholder: facets?.accounts.length ? "Conta" : "Nenhuma conta",
        options:
          facets?.accounts.map((account) => ({
            value: account.id,
            label:
              [account.name, account.institution].filter(Boolean).join(" · ") ||
              account.id,
          })) ?? [],
        formatActive: (_value, _values, option) => option?.label ?? "Conta selecionada",
      },
      {
        key: "category_id",
        label: "Categoria",
        type: "select",
        placement: "advanced",
        placeholder: facets?.categories.length ? "Categoria" : "Nenhuma categoria",
        options: categoryFilterOptions(facets?.categories),
        optionRender: (option) => (
          <span>
            <span style={{ color: option.color }} aria-hidden="true">
              {renderCategoryIcon(option.icon)}
            </span>{" "}
            {option.label}
          </span>
        ),
        formatActive: (_value, _values, option) => option?.label ?? "Categoria selecionada",
      },
      {
        key: "institution",
        label: "Instituição",
        type: "select",
        placement: "advanced",
        placeholder: facets?.institutions.length
          ? "Todas as instituições"
          : "Nenhuma instituição",
        options: facets?.institutions.map((value) => ({ value, label: value })) ?? [],
        formatActive: (value) => String(value),
      },
      {
        key: "classification",
        label: "Movimentação",
        type: "segmented",
        placement: "advanced",
        options: [
          { value: "all", label: "Todas" },
          { value: "inflow", label: "Entradas" },
          { value: "outflow", label: "Saídas" },
        ],
        formatActive: (value) =>
          classificationLabel[value as "inflow" | "outflow" | "unclassified"],
      },
      {
        key: "provider_status",
        label: "Situação",
        type: "select",
        placement: "advanced",
        placeholder: "Todas as situações",
        options: [
          { value: "POSTED", label: "Confirmada" },
          { value: "PENDING", label: "Pendente" },
        ],
        formatActive: (_value, _values, option) => option?.label ?? "Situação",
      },
      {
        key: "amount_min",
        secondaryKey: "amount_max",
        label: "Faixa de valor",
        type: "value-range",
        placement: "advanced",
        formatActive: (_value, values) =>
          `Valor: R$ ${values.amount_min ?? "0"} a R$ ${values.amount_max ?? "∞"}`,
      },
      {
        key: "uncategorized",
        label: "Somente transações sem categoria",
        type: "boolean",
        placement: "advanced",
        formatActive: () => "Sem categoria",
      },
    ],
    [facets],
  );

  return (
    <ConfigurableFilters
      label="Filtros de transações"
      end={end}
      values={applied}
      emptyValues={emptyValues ?? applied}
      config={config}
      onApply={(next) =>
        onApply({
          ...next,
          description: next.description?.trim() || null,
          classification:
            (next.classification as string | null) === "all"
              ? null
              : next.classification,
          uncategorized: next.uncategorized || null,
          credit_card: next.credit_card || null,
          card_balance: next.card_balance || null,
        })
      }
      onClear={onClear}
    />
  );
}
