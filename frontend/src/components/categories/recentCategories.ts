/**
 * The handful of categories the person picked last, kept in the browser so
 * the picker can offer them before the full list. Per-device convenience,
 * not state: storage may be missing or blocked, and then there simply are
 * no recents.
 */
const storageKey = "julius.recentCategories";
const limit = 5;

export function readRecentCategories(): string[] {
  try {
    const raw = window.localStorage.getItem(storageKey);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === "string") : [];
  } catch {
    return [];
  }
}

export function rememberRecentCategory(id: string): string[] {
  const next = [id, ...readRecentCategories().filter((other) => other !== id)].slice(0, limit);
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(next));
  } catch {
    // Storage unavailable: the pick still applies, it just isn't remembered.
  }
  return next;
}
