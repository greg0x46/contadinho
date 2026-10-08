import { App as AntdApp, ConfigProvider } from "antd";
import ptBR from "antd/locale/pt_BR";
import { useMemo, type ReactNode } from "react";

import { useCompactScreen } from "../components/shared/useCompactScreen";
import { createTheme } from "../theme/tokens";

/**
 * antd's ConfigProvider with the app theme, plus antd's `<App>` so that
 * message/notification calls (see `useFeedback`) inherit the theme and the
 * pt-BR locale. The theme follows the viewport: compact screens get
 * touch-sized controls.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const compact = useCompactScreen();
  const theme = useMemo(() => createTheme(compact), [compact]);
  return (
    <ConfigProvider locale={ptBR} theme={theme}>
      {/* component={false}: no wrapper div, so layout CSS is unaffected. */}
      <AntdApp component={false}>{children}</AntdApp>
    </ConfigProvider>
  );
}
