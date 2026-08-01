import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { createCategory, listCategories, updateCategory } from "../api/categories";
import type { CategoryCreate, CategoryUpdate } from "../api/contracts";

export const categoriesQueryKey = ["categories"] as const;

export function useCategories() {
  const queryClient = useQueryClient();
  const categoriesQuery = useQuery({
    queryKey: categoriesQueryKey,
    queryFn: ({ signal }) => listCategories(signal),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: categoriesQueryKey });

  const createMutation = useMutation({
    mutationFn: (write: CategoryCreate) => createCategory(write),
    onSuccess: invalidate,
  });

  const updateMutation = useMutation({
    mutationFn: ({ categoryId, write }: { categoryId: string; write: CategoryUpdate }) =>
      updateCategory(categoryId, write),
    onSuccess: invalidate,
  });

  return {
    categories: categoriesQuery.data ?? [],
    isLoading: categoriesQuery.isLoading,
    error: categoriesQuery.error,
    refetch: categoriesQuery.refetch,
    createCategory: createMutation.mutateAsync,
    updateCategory: updateMutation.mutateAsync,
    isSaving: createMutation.isPending || updateMutation.isPending,
  };
}
