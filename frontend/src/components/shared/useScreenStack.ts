import { useCallback, useState } from "react";

/**
 * A tiny in-memory navigation stack for a panel that swaps its own content
 * instead of opening another layer on top of itself (see PanelStack).
 *
 * `T` is the caller's screen descriptor — usually a discriminated union so
 * each screen can carry the data it was pushed with. The root is always
 * kept: `pop` on the root is a no-op, closing the panel is the container's
 * job.
 */
export function useScreenStack<T>(root: T) {
  const [stack, setStack] = useState<T[]>([root]);

  const push = useCallback((screen: T) => setStack((current) => [...current, screen]), []);
  const pop = useCallback(
    () => setStack((current) => (current.length > 1 ? current.slice(0, -1) : current)),
    [],
  );
  const replace = useCallback(
    (screen: T) => setStack((current) => [...current.slice(0, -1), screen]),
    [],
  );
  const reset = useCallback(
    () => setStack((current) => (current.length > 1 ? [current[0]!] : current)),
    [],
  );

  return {
    current: stack[stack.length - 1]!,
    depth: stack.length,
    push,
    pop,
    replace,
    reset,
  };
}
