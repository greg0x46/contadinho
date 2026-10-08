import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { createCategory, listCategories, updateCategory } from "../api/categories";
import type { CategoryCreate, CategoryUpdate } from "../api/contracts";
import { invalidateCategories, queryKeys } from "../api/queryKeys";

export const categoriesQueryKey = queryKeys.categories;

export function useCategories() {
  const queryClient = useQueryClient();
  const categoriesQuery = useQuery({
    queryKey: categoriesQueryKey,
    queryFn: ({ signal }) => listCategories(signal),
  });

  const invalidate = () => invalidateCategories(queryClient);

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
