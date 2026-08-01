import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import * as categoriesApi from "../api/categories";
import type { Category } from "../api/contracts";
import { useCategories } from "./useCategories";

vi.mock("../api/categories");

const categoryId = "33333333-3333-4333-8333-333333333333";

const category: Category = {
  id: categoryId,
  name: "Alimentação",
  kind: "expense",
  is_active: true,
  created_at: "2026-07-31T00:00:00Z",
  updated_at: "2026-07-31T00:00:00Z",
};

function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

describe("useCategories", () => {
  it("lists categories from the backend", async () => {
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    const { wrapper } = setup();
    const { result } = renderHook(useCategories, { wrapper });
    await waitFor(() => expect(result.current.categories).toEqual([category]));
  });

  it("creates a category and invalidates the list", async () => {
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([]);
    vi.mocked(categoriesApi.createCategory).mockResolvedValue(category);
    const { client, wrapper } = setup();
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(useCategories, { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(() => result.current.createCategory({ name: category.name, kind: category.kind }));

    expect(categoriesApi.createCategory).toHaveBeenCalledWith({
      name: category.name,
      kind: category.kind,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["categories"] });
  });

  it("renames and toggles a category", async () => {
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    vi.mocked(categoriesApi.updateCategory).mockResolvedValue({ ...category, name: "Renomeada" });
    const { wrapper } = setup();
    const { result } = renderHook(useCategories, { wrapper });
    await waitFor(() => expect(result.current.categories).toEqual([category]));

    await act(() =>
      result.current.updateCategory({ categoryId, write: { name: "Renomeada" } }),
    );
    expect(categoriesApi.updateCategory).toHaveBeenCalledWith(categoryId, { name: "Renomeada" });

    await act(() =>
      result.current.updateCategory({ categoryId, write: { is_active: false } }),
    );
    expect(categoriesApi.updateCategory).toHaveBeenCalledWith(categoryId, { is_active: false });
  });
});
