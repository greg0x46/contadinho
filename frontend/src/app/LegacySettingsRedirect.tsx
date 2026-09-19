import { Navigate, useLocation } from "react-router-dom";

export function LegacySettingsRedirect() {
  const { pathname, search, hash } = useLocation();
  return <Navigate replace to={{ pathname: `/configuracoes${pathname}`, search, hash }} />;
}
