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
