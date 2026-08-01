import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Flex } from "antd";
import type { ReactNode } from "react";

import { getSetupStatus } from "../api/setup";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { SetupPage } from "../pages/SetupPage";
import { UnlockPage } from "../pages/UnlockPage";

const setupStatusQueryKey = ["setup-status"] as const;

// SetupGate mirrors register_spa_host's absence in the Python reference: that
// backend read Pluggy credentials from a .env file at process start, so there
// was never a state where the app was "not yet configured". This one stores
// them encrypted in SQLite behind a password (see internal/settings), so the
// frontend needs a gate in front of every other page: configure once, then
// unlock again after every server restart.
export function SetupGate({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: setupStatusQueryKey,
    queryFn: ({ signal }) => getSetupStatus(signal),
  });

  const handleDone = () => {
    queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });
  };

  if (isLoading) {
    return (
      <Flex justify="center" align="center" style={{ minHeight: "100vh" }}>
        <LoadingState>Verificando configuração…</LoadingState>
      </Flex>
    );
  }
  if (error || data === undefined) {
    return (
      <Flex justify="center" align="center" style={{ minHeight: "100vh" }}>
        <UnavailableState onRetry={() => refetch()}>
          Não foi possível verificar a configuração da aplicação.
        </UnavailableState>
      </Flex>
    );
  }
  if (!data.configured) {
    return <SetupPage onDone={handleDone} />;
  }
  if (!data.unlocked) {
    return <UnlockPage onDone={handleDone} />;
  }
  return <>{children}</>;
}
