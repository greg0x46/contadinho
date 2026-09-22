export function filtersToSearchParams<Values extends object>(
  values: Values,
): URLSearchParams {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (Array.isArray(value) && value.length) params.set(key, value.join(","));
    else if (typeof value === "boolean" && value) params.set(key, "true");
    else if (typeof value === "string" && value) params.set(key, value);
  }
  return params;
}

/**
 * Reads an "any of" set back from the URL: the comma-joined list under
 * `key` (what filtersToSearchParams writes), or the older single-value
 * `legacyKey` when only that one is present.
 */
export function listFromSearchParams(
  params: URLSearchParams,
  key: string,
  legacyKey?: string,
): string[] {
  const raw = params.get(key) ?? (legacyKey ? params.get(legacyKey) : null);
  if (!raw) return [];
  return [...new Set(raw.split(",").map((item) => item.trim()).filter(Boolean))];
}
