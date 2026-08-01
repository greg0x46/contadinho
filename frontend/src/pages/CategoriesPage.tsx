import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useState } from "react";

import type { Category, CategoryKind } from "../api/contracts";
import { CategoryForm } from "../components/categories/CategoryForm";
import { CategoryList } from "../components/categories/CategoryList";
import { useCategories } from "../hooks/useCategories";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar a categoria.";
}

export function CategoriesPage() {
  const categories = useCategories();
  const [formOpen, setFormOpen] = useState(false);
  const [editingCategory, setEditingCategory] = useState<Category | null>(null);
  const [togglingCategoryId, setTogglingCategoryId] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const openCreate = () => {
    setEditingCategory(null);
    setSaveError(null);
    setFormOpen(true);
  };

  const openEdit = (category: Category) => {
    setEditingCategory(category);
    setSaveError(null);
    setFormOpen(true);
  };

  const closeForm = () => setFormOpen(false);

  const submit = async (draft: { name: string; kind: CategoryKind }) => {
    setSaveError(null);
    try {
      if (editingCategory) {
        await categories.updateCategory({
          categoryId: editingCategory.id,
          write: { name: draft.name },
        });
      } else {
        await categories.createCategory({ name: draft.name, kind: draft.kind });
      }
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error));
    }
  };

  const toggle = async (category: Category, isActive: boolean) => {
    setActionError(null);
    setTogglingCategoryId(category.id);
    try {
      await categories.updateCategory({ categoryId: category.id, write: { is_active: isActive } });
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setTogglingCategoryId(null);
    }
  };

  return (
    <PageContainer
      title="Categorias"
      subTitle="Catálogo interno de categorias financeiras"
      content="Gerencie as categorias usadas para classificar despesas, receitas e transferências. Categorias nunca são excluídas — apenas renomeadas ou desativadas."
      extra={[
        <Button
          key="new-category"
          type="primary"
          icon={<PlusOutlined aria-hidden="true" />}
          onClick={openCreate}
        >
          Nova categoria
        </Button>,
      ]}
    >
      {actionError && (
        <Alert
          type="error"
          showIcon
          closable
          onClose={() => setActionError(null)}
          message={actionError}
          style={{ marginBottom: 16 }}
        />
      )}
      {categories.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as categorias"
          description={<Button onClick={() => categories.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      <CategoryList
        categories={categories.categories}
        isLoading={categories.isLoading}
        togglingCategoryId={togglingCategoryId}
        onRename={openEdit}
        onToggle={toggle}
      />
      <CategoryForm
        open={formOpen}
        category={editingCategory}
        submitting={categories.isSaving}
        submitError={saveError}
        onSubmit={submit}
        onCancel={closeForm}
      />
    </PageContainer>
  );
}
