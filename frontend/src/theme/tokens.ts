import type { ThemeConfig } from "antd";

/**
 * Design tokens for Julius.
 *
 * These formalize the palette that had already emerged organically across
 * the app's CSS files (navy text, slate-gray secondary text, green primary,
 * green/red for positive/negative money, orange for warnings) into a single
 * source of truth consumed by antd's ConfigProvider. New CSS and components
 * should reference these instead of hardcoding hex values.
 */
export const colors = {
  textPrimary: "#172b4d",
  // 5.2:1 on the page background — the floor for any meaningful secondary text.
  textSecondary: "#5d697c",
  // Only on white and only for non-essential hints; fails AA on the grey bg.
  textTertiary: "#6b778c",
  // Brand green, as a fill: marks, meters, chart, active indicator. Never
  // use it for text on white (2.8:1) — use `primary` for that.
  accent: "#1baf7a",
  // Interactive green (buttons, links, active tab, focus): 5.1:1 on white.
  primary: "#0f7d58",
  primaryHover: "#0b6a4a",
  // Money coming in and "Quitada"-style success text.
  success: "#1f6f46",
  // The single red; replaces the older #a62920 / #e34948 / #f5222d.
  error: "#c4302b",
  // Warning as a fill (meters, focus ring) and as text on white.
  warning: "#f2a900",
  warningText: "#8a5a00",
  // Informational, deliberately not green so it never reads as "success".
  info: "#3b5b8c",
  // Light tint of the brand green for "selected" rows in menus and selects;
  // paired with the normal text colour, never with green text.
  selectedTint: "#eef6f2",
  selectedTintHover: "#e4f1ea",
  // Backgrounds of the StatusTag tones; each pairs with its text colour at
  // >= 4.5:1 (neutral 4.9, info 5.9, success 5.4, warning 5.3, danger 4.7).
  tagNeutralBg: "#eef0f4",
  tagInfoBg: "#e9eff8",
  tagSuccessBg: "#e5f3ec",
  tagWarningBg: "#fcf1d6",
  tagDangerBg: "#fbe8e7",
  // Row hover on a white surface (mirrors --color-hover).
  hover: "rgba(23, 43, 77, 0.035)",
  border: "#d9d9d9",
  borderSecondary: "#e5e9f0",
  bgLayout: "#f5f7fa",
  bgContainer: "#ffffff",
} as const;

export const fontFamily =
  'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

/** 4px baseline spacing scale, exposed for use outside antd's own Space/Flex props. */
export const spacing = {
  xs: 4,
  sm: 8,
  md: 16,
  lg: 24,
  xl: 32,
  xxl: 48,
} as const;

/** Control heights on a pointer-friendly (compact) screen vs. a desktop one. */
const controlHeights = {
  wide: { controlHeight: 32, controlHeightLG: 40, controlHeightSM: 24 },
  compact: { controlHeight: 40, controlHeightLG: 48, controlHeightSM: 32 },
} as const;

/**
 * The antd theme. On compact screens the controls grow to touch size
 * (40/48/32); the 16px input font that stops iOS from zooming on focus is
 * CSS-only (global.css), since antd has no per-control token for it.
 */
export function createTheme(compact: boolean): ThemeConfig {
  return {
    token: {
      colorPrimary: colors.primary,
      colorLink: colors.primary,
      colorLinkHover: colors.primaryHover,
      colorInfo: colors.info,
      colorSuccess: colors.success,
      colorError: colors.error,
      colorWarning: colors.warning,
      colorWarningText: colors.warningText,
      colorTextBase: colors.textPrimary,
      colorTextSecondary: colors.textSecondary,
      colorTextTertiary: colors.textTertiary,
      // 4.6:1 on white; antd's default (25% black) is 1.65:1.
      colorTextPlaceholder: colors.textTertiary,
      colorBorder: colors.border,
      colorBorderSecondary: colors.borderSecondary,
      colorBgLayout: colors.bgLayout,
      colorBgContainer: colors.bgContainer,
      fontFamily,
      borderRadius: 8,
      borderRadiusLG: 8,
      borderRadiusSM: 6,
      fontSize: 14,
      // Surfaces are separated by hairlines, never shadows (the Segmented
      // thumb reads off a hairline ring instead, see global.css).
      boxShadowTertiary: "none",
      // A selected option in any menu or select: a light tint with the
      // normal text colour, instead of a green-grey fill.
      controlItemBgActive: colors.selectedTint,
      controlItemBgActiveHover: colors.selectedTintHover,
      ...(compact ? controlHeights.compact : controlHeights.wide),
    },
    components: {
      // Flat buttons: antd paints a soft coloured shadow under primary,
      // default and danger buttons that no other surface in the app has.
      Button: {
        primaryShadow: "none",
        defaultShadow: "none",
        dangerShadow: "none",
      },
      Card: {
        borderRadiusLG: 8,
      },
      Select: {
        optionSelectedBg: colors.selectedTint,
        optionSelectedColor: colors.textPrimary,
        optionSelectedFontWeight: 500,
        optionActiveBg: colors.hover,
      },
      // Position and colour are the cue; the thumb's green-tinted drop
      // shadow isn't needed on a white handle and breaks the flat language.
      Switch: {
        handleShadow: "none",
      },
      // A slightly darker track than the page lets the white selected thumb
      // read as a thumb without a shadow (a hairline ring: global.css).
      Segmented: {
        trackBg: "#eceff4",
        itemSelectedBg: colors.bgContainer,
      },
      // ONE table skin for every table (the 12px header label is
      // global.css): white header with secondary-colour labels, hairlines
      // only, a faint hover. Domain stylesheets must not restyle headers.
      Table: {
        headerBg: colors.bgContainer,
        headerColor: colors.textSecondary,
        headerSplitColor: "transparent",
        headerBorderRadius: 0,
        borderColor: colors.borderSecondary,
        rowHoverBg: colors.hover,
        cellPaddingBlock: 12,
      },
      Layout: {
        headerBg: colors.bgContainer,
        bodyBg: colors.bgLayout,
      },
    },
  };
}

/** The wide-screen theme, for callers that don't track the viewport. */
export const theme: ThemeConfig = createTheme(false);
