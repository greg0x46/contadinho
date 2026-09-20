import { Grid } from "antd";

/**
 * Whether the viewport is below antd's `md` breakpoint — the point where a
 * side panel stops fitting beside the page and becomes a full-screen page,
 * and a dropdown gives way to a bottom sheet. `md` is undefined before the
 * first media-query tick; that counts as compact so a phone never flashes
 * the desktop layout.
 */
export function useCompactScreen(): boolean {
  const screens = Grid.useBreakpoint();
  return !screens.md;
}
