import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { createAppQueryClient } from "../app/queryClient";

export function QueryTestProvider({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={createAppQueryClient()}>{children}</QueryClientProvider>;
}
