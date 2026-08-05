import { useMemo } from "react";

import type {
  TransactionFilters as Filters,
  TransactionQueryResult,
} from "../../api/contracts";
import {
  ConfigurableFilters,
  type DateRangePreset,
  type FilterConfig,
} from "../filters/ConfigurableFilters";
import { categoryFilterOptions, renderCategoryIcon } from "../../presentation/categoryLabels";
import { classificationLabel } from "../../presentation/transactionStatus";

function dateText(date: Date): string {
  return [
    String(date.getFullYear()).padStart(4, "0"),
    String(date.getMonth() + 1).padStart(2, "0"),
    String(date.getDate()).padStart(2, "0"),
  ].join("-");
}

function monthRange(offset: number): [string, string] {
  const today = new Date();
  const start = new Date(today.getFullYear(), today.getMonth() + offset, 1);
  const end = new Date(today.getFullYear(), today.getMonth() + offset + 1, 0);
  return [dateText(start), dateText(end)];
}

function periodPresets(): DateRangePreset[] {
  return [
    { value: "this-month", label: "Este mês", range: () => monthRange(0) },
    { value: "last-month", label: "Mês passado", range: () => monthRange(-1) },
    {
      value: "last-30-days",
      label: "Últimos 30 dias",
      range: () => {
        const end = new Date();
        const start = new Date(end.getFullYear(), end.getMonth(), end.getDate() - 29);
        return [dateText(start), dateText(end)];
      },
    },
    {
      value: "this-year",
      label: "Este ano",
      range: () => {
        const today = new Date();
        return [
          dateText(new Date(today.getFullYear(), 0, 1)),
          dateText(new Date(today.getFullYear(), 11, 31)),
        ];
      },
    },
  ];
}

export function TransactionFilters({
  applied,
  emptyValues,
  facets,
  onApply,
  onClear,
}: {
  applied: Filters;
  emptyValues?: Filters;
  facets: TransactionQueryResult["available_filters"] | undefined;
  onApply: (filters: Filters) => void;
  onClear: () => void;
}) {
  const config = useMemo<FilterConfig<Filters>[]>(
    () => [
      {
        key: "date_from",
        secondaryKey: "date_to",
        label: "Período",
        type: "date-range",
        placement: "main",
        presets: periodPresets(),
        hideChip: true,
      },
      {
        key: "description",
        label: "Descrição",
        type: "text",
        placement: "main",
        placeholder: "Buscar descrição",
        debounceMs: 300,
        formatActive: (value) => String(value),
      },
      {
        key: "account_id",
        label: "Conta",
        type: "select",
        placement: "main",
        placeholder: facets?.accounts.length ? "Todas as contas" : "Nenhuma conta",
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
        placement: "main",
        placeholder: facets?.categories.length ? "Todas as categorias" : "Nenhuma categoria",
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
        label: "Classificação",
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
        })
      }
      onClear={onClear}
    />
  );
}
