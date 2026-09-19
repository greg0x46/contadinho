import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createInvestmentAccount,
  createInvestmentOperation,
  createInvestmentOperations,
  createInvestmentTransfer,
  type InvestmentTransferWrite,
  type InvestmentCompoundWrite,
  createInvestmentPortfolio,
  createInvestmentPosition,
  createInvestmentReconciliation,
  deleteInvestmentAccount,
  deleteInvestmentOperation,
  deleteInvestmentPortfolio,
  deleteInvestmentPosition,
  deleteInvestmentReconciliation,
  deleteInvestmentTransfer,
  getInvestmentSummary,
  listInvestmentAccounts,
  listInvestmentOperations,
  listInvestmentPortfolios,
  listInvestmentPositions,
  listInvestmentReconciliations,
  updateInvestmentAccount,
  updateInvestmentOperation,
  updateInvestmentPortfolio,
  updateInvestmentPosition,
} from "../api/investmentPortfolio";
import type {
  InvestmentAccountUpdate,
  InvestmentAccountWrite,
  InvestmentOperationWrite,
  InvestmentPortfolioWrite,
  InvestmentPositionUpdate,
  InvestmentPositionWrite,
  InvestmentReconciliationWrite,
} from "../api/contracts";

export const investmentWorkspaceQueryKey = ["investmentWorkspace"] as const;

/**
 * The investment ledger has a deliberately small, connected set of views.
 * One workspace cache keeps a new operation, its cash balance, the goal and
 * the summary moving together instead of leaving stale figures in another
 * tab of the same page.
 */
export function useInvestmentWorkspace({ enabled = true }: { enabled?: boolean } = {}) {
  const queryClient = useQueryClient();
  // `enabled` lets a component that only sometimes needs the ledger (the
  // transaction drawer's reconciliation panel) mount without firing six
  // requests; mutations stay available regardless.
  const accountsQuery = useQuery({
    queryKey: [...investmentWorkspaceQueryKey, "accounts"],
    queryFn: ({ signal }) => listInvestmentAccounts(signal),
    enabled,
  });
  const portfoliosQuery = useQuery({
    queryKey: [...investmentWorkspaceQueryKey, "portfolios"],
    queryFn: ({ signal }) => listInvestmentPortfolios(signal),
    enabled,
  });
  const positionsQuery = useQuery({
    queryKey: [...investmentWorkspaceQueryKey, "positions"],
    queryFn: ({ signal }) => listInvestmentPositions({ includeClosed: true }, signal),
    enabled,
  });
  const operationsQuery = useQuery({
    queryKey: [...investmentWorkspaceQueryKey, "operations"],
    queryFn: ({ signal }) => listInvestmentOperations({}, signal),
    enabled,
  });
  const reconciliationsQuery = useQuery({
    queryKey: [...investmentWorkspaceQueryKey, "reconciliations"],
    queryFn: ({ signal }) => listInvestmentReconciliations({}, signal),
    enabled,
  });
  const summaryQuery = useQuery({
    queryKey: [...investmentWorkspaceQueryKey, "summary"],
    queryFn: ({ signal }) => getInvestmentSummary(signal),
    enabled,
  });

  const refreshLedger = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: investmentWorkspaceQueryKey }),
      // A reconciliation changes what is reported as ordinary cash flow.
      queryClient.invalidateQueries({ queryKey: ["transactions"] }),
      queryClient.invalidateQueries({ queryKey: ["timeline"] }),
      queryClient.invalidateQueries({ queryKey: ["netWorth"] }),
    ]);
  };

  const createAccountMutation = useMutation({
    mutationFn: (write: InvestmentAccountWrite) => createInvestmentAccount(write),
    onSuccess: refreshLedger,
  });
  const updateAccountMutation = useMutation({
    mutationFn: ({ accountId, write }: { accountId: string; write: InvestmentAccountUpdate }) =>
      updateInvestmentAccount(accountId, write),
    onSuccess: refreshLedger,
  });
  const deleteAccountMutation = useMutation({
    mutationFn: (accountId: string) => deleteInvestmentAccount(accountId),
    onSuccess: refreshLedger,
  });

  const createPortfolioMutation = useMutation({
    mutationFn: (write: InvestmentPortfolioWrite) => createInvestmentPortfolio(write),
    onSuccess: refreshLedger,
  });
  const updatePortfolioMutation = useMutation({
    mutationFn: ({ portfolioId, write }: { portfolioId: string; write: InvestmentPortfolioWrite }) =>
      updateInvestmentPortfolio(portfolioId, write),
    onSuccess: refreshLedger,
  });
  const deletePortfolioMutation = useMutation({
    mutationFn: (portfolioId: string) => deleteInvestmentPortfolio(portfolioId),
    onSuccess: refreshLedger,
  });

  const createPositionMutation = useMutation({
    mutationFn: (write: InvestmentPositionWrite) => createInvestmentPosition(write),
    onSuccess: refreshLedger,
  });
  const updatePositionMutation = useMutation({
    mutationFn: ({ positionId, write }: { positionId: string; write: InvestmentPositionUpdate }) =>
      updateInvestmentPosition(positionId, write),
    onSuccess: refreshLedger,
  });
  const deletePositionMutation = useMutation({
    mutationFn: (positionId: string) => deleteInvestmentPosition(positionId),
    onSuccess: refreshLedger,
  });

  const createOperationMutation = useMutation({
    mutationFn: (write: InvestmentOperationWrite) => createInvestmentOperation(write),
    onSuccess: refreshLedger,
  });
  const createOperationsMutation = useMutation({
    mutationFn: (write: InvestmentCompoundWrite) => createInvestmentOperations(write),
    onSuccess: refreshLedger,
  });
  const updateOperationMutation = useMutation({
    mutationFn: ({ operationId, write }: { operationId: string; write: InvestmentOperationWrite }) =>
      updateInvestmentOperation(operationId, write),
    onSuccess: refreshLedger,
  });
  const deleteOperationMutation = useMutation({
    mutationFn: (operationId: string) => deleteInvestmentOperation(operationId),
    onSuccess: refreshLedger,
  });
  const createTransferMutation = useMutation({
    mutationFn: (write: InvestmentTransferWrite) => createInvestmentTransfer(write),
    onSuccess: refreshLedger,
  });
  const deleteTransferMutation = useMutation({
    mutationFn: (transferId: string) => deleteInvestmentTransfer(transferId),
    onSuccess: refreshLedger,
  });

  const createReconciliationMutation = useMutation({
    mutationFn: (write: InvestmentReconciliationWrite) => createInvestmentReconciliation(write),
    onSuccess: refreshLedger,
  });
  const deleteReconciliationMutation = useMutation({
    mutationFn: (reconciliationId: string) => deleteInvestmentReconciliation(reconciliationId),
    onSuccess: refreshLedger,
  });

  const refetch = async () => {
    await Promise.all([
      accountsQuery.refetch(),
      portfoliosQuery.refetch(),
      positionsQuery.refetch(),
      operationsQuery.refetch(),
      reconciliationsQuery.refetch(),
      summaryQuery.refetch(),
    ]);
  };

  return {
    accounts: accountsQuery.data ?? [],
    portfolios: portfoliosQuery.data ?? [],
    positions: positionsQuery.data ?? [],
    operations: operationsQuery.data ?? [],
    reconciliations: reconciliationsQuery.data ?? [],
    summary: summaryQuery.data ?? null,
    isLoading:
      accountsQuery.isLoading ||
      portfoliosQuery.isLoading ||
      positionsQuery.isLoading ||
      operationsQuery.isLoading ||
      reconciliationsQuery.isLoading ||
      summaryQuery.isLoading,
    isFetching:
      accountsQuery.isFetching ||
      portfoliosQuery.isFetching ||
      positionsQuery.isFetching ||
      operationsQuery.isFetching ||
      reconciliationsQuery.isFetching ||
      summaryQuery.isFetching,
    error:
      accountsQuery.error ??
      portfoliosQuery.error ??
      positionsQuery.error ??
      operationsQuery.error ??
      reconciliationsQuery.error ??
      summaryQuery.error,
    refetch,
    createAccount: createAccountMutation.mutateAsync,
    updateAccount: updateAccountMutation.mutateAsync,
    deleteAccount: deleteAccountMutation.mutateAsync,
    createPortfolio: createPortfolioMutation.mutateAsync,
    updatePortfolio: updatePortfolioMutation.mutateAsync,
    deletePortfolio: deletePortfolioMutation.mutateAsync,
    createPosition: createPositionMutation.mutateAsync,
    updatePosition: updatePositionMutation.mutateAsync,
    deletePosition: deletePositionMutation.mutateAsync,
    createOperation: createOperationMutation.mutateAsync,
    createTransfer: createTransferMutation.mutateAsync,
    deleteTransfer: deleteTransferMutation.mutateAsync,
    createOperations: createOperationsMutation.mutateAsync,
    updateOperation: updateOperationMutation.mutateAsync,
    deleteOperation: deleteOperationMutation.mutateAsync,
    createReconciliation: createReconciliationMutation.mutateAsync,
    deleteReconciliation: deleteReconciliationMutation.mutateAsync,
    isSaving:
      createAccountMutation.isPending ||
      updateAccountMutation.isPending ||
      createPortfolioMutation.isPending ||
      updatePortfolioMutation.isPending ||
      createPositionMutation.isPending ||
      updatePositionMutation.isPending ||
      createOperationMutation.isPending ||
      createTransferMutation.isPending ||
      createOperationsMutation.isPending ||
      updateOperationMutation.isPending ||
      createReconciliationMutation.isPending,
    isDeleting:
      deleteAccountMutation.isPending ||
      deletePortfolioMutation.isPending ||
      deletePositionMutation.isPending ||
      deleteOperationMutation.isPending ||
      deleteTransferMutation.isPending ||
      deleteReconciliationMutation.isPending,
  };
}
