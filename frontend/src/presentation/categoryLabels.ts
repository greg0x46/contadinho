import type { CategoryKind, CategoryOption, InternalCategory, TransactionItem } from "../api/contracts";
import type { FilterOption } from "../components/filters/ConfigurableFilters";

export const categoryKindLabel: Record<CategoryKind, string> = {
  expense: "Despesa",
  income: "Receita",
  transfer: "Transferência",
};

export const categoryKindColor: Record<CategoryKind, string> = {
  expense: "red",
  income: "green",
  transfer: "blue",
};

export function internalCategoryOriginLabel(category: InternalCategory): string {
  return category.origin === "manual" ? "Categorização manual" : "Categorização automática";
}

export function internalCategoryName(item: Pick<TransactionItem, "internal_category">): string {
  return item.internal_category?.name ?? "Sem categoria";
}

export function categoryFilterOptions(categories: CategoryOption[] | undefined): FilterOption[] {
  if (!categories) return [];
  const byKind = ["expense", "income", "transfer"] as const;
  const sorted = [...categories].sort((a, b) => {
    if (a.kind !== b.kind) return byKind.indexOf(a.kind) - byKind.indexOf(b.kind);
    if (a.is_active !== b.is_active) return a.is_active ? -1 : 1;
    return a.name.localeCompare(b.name, "pt-BR");
  });
  return sorted.map((option) => ({
    value: option.id,
    label: `${categoryKindLabel[option.kind]}: ${option.name}${option.is_active ? "" : " (inativa)"}`,
  }));
}
