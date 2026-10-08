import { useState } from "react";

import type { TransactionItem } from "../../api/contracts";

/**
 * The line whose panel is open, kept as a snapshot.
 *
 * Deriving it from the current page of results closed the panel whenever a
 * write moved the line out of the filter being looked at (categorizing under
 * "Sem categoria", ignoring a line, a category filter): the line simply
 * vanished from `items`. The snapshot keeps the panel on screen and follows
 * fresh data whenever the line is still in the results, so the panel never
 * shows a decision the server has already replaced.
 */
export function useSelectedTransaction(items: TransactionItem[] | undefined) {
  const [snapshot, setSnapshot] = useState<TransactionItem | null>(null);
  // Fresh data is adopted only when a new page of results arrives. Comparing
  // against the snapshot on every render would put the old cached copy back
  // right after `patch`, while the refetch that follows a write is in flight.
  const [seenItems, setSeenItems] = useState(items);
  if (!sameItems(items, seenItems)) {
    setSeenItems(items);
    const fresh = snapshot === null ? undefined : items?.find((item) => item.id === snapshot.id);
    if (fresh !== undefined && fresh !== snapshot) setSnapshot(fresh);
  }

  return {
    selected: snapshot,
    /**
     * Records a confirmed write on the snapshot. When the write moved the line
     * out of the results there is no fresh copy to follow, and the panel must
     * still show the decision just made, not the one before it.
     */
    patch: (id: string, update: (item: TransactionItem) => TransactionItem) =>
      setSnapshot((current) => (current !== null && current.id === id ? update(current) : current)),
    select: (id: string | null) => {
      if (id === null) {
        setSnapshot(null);
        return;
      }
      const item = items?.find((candidate) => candidate.id === id);
      if (item) setSnapshot(item);
    },
  };
}

/**
 * Same page of results: the same line objects in the same order. Callers often
 * pass `data?.items ?? []`, a new empty array on every render while loading,
 * so array identity alone would read as "new results" forever.
 */
function sameItems(a: TransactionItem[] | undefined, b: TransactionItem[] | undefined) {
  if (a === b) return true;
  if (a === undefined || b === undefined || a.length !== b.length) return false;
  return a.every((item, index) => item === b[index]);
}
