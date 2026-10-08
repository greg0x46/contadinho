import { isUuid, isValidDate, requiredRecord } from "./shared";

export const categoryKinds = ["expense", "income", "transfer"] as const;

export type CategoryKind = (typeof categoryKinds)[number];

// "rule" mirrors categories.OriginRule (internal/categories/decisions.go):
// set when a matching automation rule's set_category action assigns the
// category, distinct from "automatic" (the source_category mapping) and
// "manual" (the user). "learned" mirrors categories.OriginLearned: the
// category was copied from a past manual decision on a similar transaction.
export const categoryOrigins = ["manual", "automatic", "rule", "learned"] as const;

export type CategoryOrigin = (typeof categoryOrigins)[number];

export interface Category {
  id: string;
  name: string;
  kind: CategoryKind;
  is_active: boolean;
  icon: string;
  color: string;
  created_at: string;
  updated_at: string;
}

export interface CategoryCreate {
  name: string;
  kind: CategoryKind;
  icon: string;
  color: string;
}

export interface CategoryUpdate {
  name?: string;
  is_active?: boolean;
  icon?: string;
  color?: string;
}

export function parseCategory(value: unknown): Category {
  const category = requiredRecord(
    value,
    ["id", "name", "kind", "is_active", "icon", "color", "created_at", "updated_at"],
    "Categoria inválida.",
  );
  if (
    typeof category.id !== "string" ||
    !isUuid(category.id) ||
    typeof category.name !== "string" ||
    category.name === "" ||
    !categoryKinds.includes(category.kind as CategoryKind) ||
    typeof category.is_active !== "boolean" ||
    typeof category.icon !== "string" ||
    category.icon === "" ||
    typeof category.color !== "string" ||
    category.color === "" ||
    !isValidDate(category.created_at) ||
    !isValidDate(category.updated_at)
  ) {
    throw new TypeError("Categoria inválida.");
  }
  return {
    id: category.id,
    name: category.name,
    kind: category.kind as CategoryKind,
    is_active: category.is_active,
    icon: category.icon,
    color: category.color,
    created_at: category.created_at as string,
    updated_at: category.updated_at as string,
  };
}

export function parseCategoryList(value: unknown): Category[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de categorias inválida.");
  }
  return value.map(parseCategory);
}
