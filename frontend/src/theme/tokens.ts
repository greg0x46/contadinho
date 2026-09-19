import type { ThemeConfig } from "antd";

/**
 * Design tokens for Contadinho.
 *
 * These formalize the palette that had already emerged organically across
 * the app's CSS files (navy text, slate-gray secondary text, green primary,
 * green/red for positive/negative money, orange for warnings) into a single
 * source of truth consumed by antd's ConfigProvider. New CSS and components
 * should reference these instead of hardcoding hex values.
 */
export const colors = {
  textPrimary: "#172b4d",
  textSecondary: "#6b778c",
  textTertiary: "#8c98a8",
  // Same tone as `success` — the theme's accent color is green.
  primary: "#1baf7a",
  success: "#1baf7a",
  error: "#e34948",
  warning: "#f2a900",
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

export const theme: ThemeConfig = {
  token: {
    colorPrimary: colors.primary,
    colorLink: colors.primary,
    colorInfo: colors.primary,
    colorSuccess: colors.success,
    colorError: colors.error,
    colorWarning: colors.warning,
    colorTextBase: colors.textPrimary,
    colorTextSecondary: colors.textSecondary,
    colorTextTertiary: colors.textTertiary,
    colorBorder: colors.border,
    colorBorderSecondary: colors.borderSecondary,
    colorBgLayout: colors.bgLayout,
    colorBgContainer: colors.bgContainer,
    fontFamily,
    borderRadius: 8,
    borderRadiusLG: 12,
    borderRadiusSM: 6,
    fontSize: 14,
  },
  components: {
    Card: {
      borderRadiusLG: 12,
      boxShadowTertiary: "0 1px 2px rgba(23, 43, 77, 0.06)",
    },
    Layout: {
      headerBg: colors.bgContainer,
      bodyBg: colors.bgLayout,
    },
  },
};
