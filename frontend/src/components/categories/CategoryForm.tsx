import { Alert, Button, Drawer, Flex, Input, Select } from "antd";
import { useEffect, useState } from "react";

import type { Category, CategoryKind } from "../../api/contracts";
import { categoryKindLabel } from "../../presentation/categoryLabels";

const kindOptions = (Object.keys(categoryKindLabel) as CategoryKind[]).map((value) => ({
  value,
  label: categoryKindLabel[value],
}));

function draftFrom(category: Category | null): { name: string; kind: CategoryKind } {
  return category
    ? { name: category.name, kind: category.kind }
    : { name: "", kind: "expense" };
}

export function CategoryForm({
  open,
  category,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  open: boolean;
  category: Category | null;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (draft: { name: string; kind: CategoryKind }) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState(() => draftFrom(category));
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setDraft(draftFrom(category));
      setError(null);
    }
  }, [open, category]);

  const submit = () => {
    if (draft.name.trim() === "") {
      setError("Informe um nome para a categoria.");
      return;
    }
    setError(null);
    onSubmit({ name: draft.name.trim(), kind: draft.kind });
  };

  return (
    <Drawer
      title={category ? "Renomear categoria" : "Nova categoria"}
      open={open}
      onClose={onCancel}
      width={420}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button onClick={onCancel}>Cancelar</Button>
          <Button type="primary" loading={submitting} onClick={submit}>
            Salvar
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="middle">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}

        <div className="filter-field">
          <label htmlFor="category-name">Nome</label>
          <Input
            id="category-name"
            value={draft.name}
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder="Ex.: Manutenção do Apartamento"
          />
        </div>

        <div className="filter-field">
          <label htmlFor="category-kind">Tipo</label>
          <Select
            id="category-kind"
            value={draft.kind}
            options={kindOptions}
            disabled={category !== null}
            onChange={(value: CategoryKind) => setDraft((current) => ({ ...current, kind: value }))}
          />
        </div>
      </Flex>
    </Drawer>
  );
}
