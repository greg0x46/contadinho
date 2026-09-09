import { useState } from "react";
import { periodPresets } from "../components/filters/periodPresets";
import { periodNavigation } from "../components/filters/periodNavigation";

/** A window, in the same shape the transactions filter uses: nulls mean "all of it". */
export type HomePeriod = { from: string; to: string } | { from: null; to: null };

const storageKey = "contadinho.home.periodo";
const defaultPreset = "this-month";

function presetHomePeriod(value: string): HomePeriod | null {
  const preset = periodPresets().find((candidate) => candidate.value === value);
  if (!preset) return null;
  const [from, to] = preset.range();
  return from !== null && to !== null ? { from, to } : { from: null, to: null };
}

function defaultHomePeriod(): HomePeriod {
  return presetHomePeriod(defaultPreset) ?? { from: null, to: null };
}

/**
 * The chosen window is a per-browser preference, not part of the page's
 * address: coming back to the Home through the menu must keep it, so it lives
 * in localStorage. Reading and writing it can throw outright — private
 * browsing, or a browser set to block site data — so neither side may be the
 * thing that breaks the dashboard.
 *
 * A window that came from a shortcut is stored as the shortcut and resolved
 * again on the way back in: "este ano" saved in December must still mean this
 * year in January, not the year it was picked.
 */
function storedHomePeriod(): HomePeriod {
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(storageKey);
  } catch {
    return defaultHomePeriod();
  }
  if (raw === null) return defaultHomePeriod();
  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) return defaultHomePeriod();
    const { preset, from, to } = parsed as Record<string, unknown>;
    if (typeof preset === "string") return presetHomePeriod(preset) ?? defaultHomePeriod();
    if (typeof from === "string" && typeof to === "string" && from >= "0001-01-01" && periodNavigation(from, to)) return { from, to };
  } catch {
    // Ignore malformed browser preferences.
  }
  return defaultHomePeriod();
}

function storeHomePeriod(period: HomePeriod): void {
  const preset = periodPresets().find((candidate) => {
    const [from, to] = candidate.range();
    return period.from === from && period.to === to;
  });
  try {
    window.localStorage.setItem(
      storageKey,
      JSON.stringify(preset ? { preset: preset.value } : period),
    );
  } catch {
    // A window that can't be remembered is still a usable window.
  }
}

export function useHomePeriod() {
  const [period, setPeriodState] = useState<HomePeriod>(storedHomePeriod);
  const setPeriod = (from: string | null, to: string | null) => {
    const next: HomePeriod = from !== null && to !== null ? { from, to } : { from: null, to: null };
    setPeriodState(next);
    storeHomePeriod(next);
  };
  return { period, setPeriod };
}
