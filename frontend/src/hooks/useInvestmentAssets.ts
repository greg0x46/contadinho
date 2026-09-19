import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createInvestmentAsset,
  deleteInvestmentAsset,
  listInvestmentAssets,
  updateInvestmentAsset,
} from "../api/investmentPortfolio";
import type { InvestmentAssetWrite } from "../api/contracts";

export const investmentAssetsQueryKey = ["investmentAssets"] as const;
const investmentWorkspaceQueryKey = ["investmentWorkspace"] as const;

export function useInvestmentAssets() {
  const queryClient = useQueryClient();
  const assetsQuery = useQuery({
    queryKey: investmentAssetsQueryKey,
    queryFn: ({ signal }) => listInvestmentAssets(signal),
  });

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: investmentAssetsQueryKey }),
      queryClient.invalidateQueries({ queryKey: investmentWorkspaceQueryKey }),
    ]);
  };

  const createMutation = useMutation({
    mutationFn: (write: InvestmentAssetWrite) => createInvestmentAsset(write),
    onSuccess: refresh,
  });
  const updateMutation = useMutation({
    mutationFn: ({ assetId, write }: { assetId: string; write: InvestmentAssetWrite }) =>
      updateInvestmentAsset(assetId, write),
    onSuccess: refresh,
  });
  const deleteMutation = useMutation({
    mutationFn: (assetId: string) => deleteInvestmentAsset(assetId),
    onSuccess: refresh,
  });

  return {
    assets: assetsQuery.data ?? [],
    isLoading: assetsQuery.isLoading,
    error: assetsQuery.error,
    refetch: assetsQuery.refetch,
    createAsset: createMutation.mutateAsync,
    updateAsset: updateMutation.mutateAsync,
    deleteAsset: deleteMutation.mutateAsync,
    isSaving: createMutation.isPending || updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
  };
}
