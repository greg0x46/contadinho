import { isNullableString, isRecord, isUuid, isValidDate } from "./shared";

/** One provider connection (a Pluggy item): the unit that owns accounts and syncs. */
export interface DataSource {
  id: string;
  provider: string;
  external_item_id: string;
  /** The institution reported by the last sync; overwritten on every run. */
  display_name: string | null;
  /** The user's own name for the connection, which wins over display_name. */
  label: string | null;
  /** What to show: label, else display_name, else the raw item id. */
  name: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export function parseDataSource(value: unknown): DataSource {
  if (
    !isRecord(value) ||
    typeof value.id !== "string" ||
    !isUuid(value.id) ||
    typeof value.provider !== "string" ||
    typeof value.external_item_id !== "string" ||
    !isNullableString(value.display_name) ||
    !isNullableString(value.label) ||
    typeof value.name !== "string" ||
    typeof value.is_active !== "boolean" ||
    !isValidDate(value.created_at) ||
    !isValidDate(value.updated_at)
  ) {
    throw new TypeError("Conexão inválida.");
  }

  return {
    id: value.id,
    provider: value.provider,
    external_item_id: value.external_item_id,
    display_name: value.display_name,
    label: value.label,
    name: value.name,
    is_active: value.is_active,
    created_at: value.created_at,
    updated_at: value.updated_at,
  };
}

export function parseDataSourceList(value: unknown): DataSource[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de conexões inválida.");
  }
  return value.map(parseDataSource);
}
