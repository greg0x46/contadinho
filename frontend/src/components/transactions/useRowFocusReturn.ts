import { useRef } from "react";

/**
 * Closing the transaction panel (Esc, ×, ←) must leave the keyboard where it
 * started: on the row that opened it. The drawer remembers the element that
 * was focused when it opened, but by then focus had already moved to the
 * panel's title, so it restored nothing useful and focus fell back to the
 * top of the page.
 *
 * `remember` is called when a row is chosen, `restore` when the panel is
 * dismissed by the person (not when a filter change closes it).
 */
export function useRowFocusReturn() {
  const lastId = useRef<string | null>(null);
  return {
    remember: (transactionId: string) => {
      lastId.current = transactionId;
    },
    restore: () => {
      const id = lastId.current;
      if (id === null) return;
      // After the drawer has handed focus back (or dropped it).
      window.setTimeout(() => {
        const row = Array.from(document.querySelectorAll<HTMLElement>("[data-transaction-id]")).find(
          (element) => element.dataset.transactionId === id,
        );
        row?.focus();
      }, 0);
    },
  };
}
