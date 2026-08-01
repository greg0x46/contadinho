import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import * as categoriesApi from "../api/categories";
import type { Category } from "../api/contracts";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { CategoriesPage } from "./CategoriesPage";

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

function renderPage() {
  return render(
    <MemoryRouter>
      <QueryTestProvider>
        <CategoriesPage />
      </QueryTestProvider>
    </MemoryRouter>,
  );
}

describe("CategoriesPage", () => {
  it("lists existing categories with their type", async () => {
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    renderPage();
    expect(await screen.findByText("Alimentação")).toBeVisible();
    expect(screen.getByText("Despesa")).toBeVisible();
  });

  it("blocks saving a new category without a name", async () => {
    const user = userEvent.setup();
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova categoria" }));

    await user.click(screen.getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("Informe um nome para a categoria.")).toBeVisible();
    expect(categoriesApi.createCategory).not.toHaveBeenCalled();
  });

  it("creates a category with the entered name and type", async () => {
    const user = userEvent.setup();
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([]);
    vi.mocked(categoriesApi.createCategory).mockResolvedValue(category);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nova categoria" }));
    await user.type(screen.getByLabelText("Nome"), category.name);
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(categoriesApi.createCategory).toHaveBeenCalledWith({
        name: category.name,
        kind: "expense",
      }),
    );
    expect(screen.queryByRole("button", { name: "Salvar" })).not.toBeInTheDocument();
  });

  it("blocks changing the type field when editing an existing category", async () => {
    const user = userEvent.setup();
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Renomear" }, { timeout: 5000 }));
    const kindField = await screen.findByRole("combobox", { name: "Tipo" });
    expect(kindField.closest(".ant-select")).toHaveClass("ant-select-disabled");
  });

  it("renames an existing category, never sending kind", async () => {
    const user = userEvent.setup();
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    vi.mocked(categoriesApi.updateCategory).mockResolvedValue({
      ...category,
      name: "Renomeada",
    });
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Renomear" }, { timeout: 5000 }));
    const nameField = await screen.findByLabelText("Nome");
    expect(nameField).toHaveValue(category.name);
    await user.clear(nameField);
    await user.type(nameField, "Renomeada");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() =>
      expect(categoriesApi.updateCategory).toHaveBeenCalledWith(categoryId, {
        name: "Renomeada",
      }),
    );
  });

  it("toggles a category active state", async () => {
    const user = userEvent.setup();
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    vi.mocked(categoriesApi.updateCategory).mockResolvedValue({
      ...category,
      is_active: false,
    });
    renderPage();
    const toggle = await screen.findByRole("switch");
    await user.click(toggle);

    await waitFor(() =>
      expect(categoriesApi.updateCategory).toHaveBeenCalledWith(categoryId, {
        is_active: false,
      }),
    );
  });

  it("never offers a delete action", async () => {
    vi.mocked(categoriesApi.listCategories).mockResolvedValue([category]);
    renderPage();
    await screen.findByText(category.name);
    expect(screen.queryByRole("button", { name: /excluir/i })).not.toBeInTheDocument();
  });
});
