import { CheckOutlined } from "@ant-design/icons";
import { Alert, Button, Drawer, Flex, Input, Select, Switch } from "antd";
import { useEffect, useState } from "react";

import type { Category, CategoryKind } from "../../api/contracts";
import {
  categoryColorPalette,
  categoryIconRegistry,
  categoryKindLabel,
  renderCategoryIcon,
} from "../../presentation/categoryLabels";

const kindOptions = (Object.keys(categoryKindLabel) as CategoryKind[]).map((value) => ({
  value,
  label: categoryKindLabel[value],
}));

const iconOptions = Object.keys(categoryIconRegistry).map((key) => ({
  value: key,
  label: (
    <Flex align="center" gap="small">
      {renderCategoryIcon(key)}
      <span>{key}</span>
    </Flex>
  ),
}));

const defaultIcon = "ellipsis";
const defaultColor = categoryColorPalette[0];

interface CategoryDraft {
  name: string;
  kind: CategoryKind;
  icon: string;
  color: string;
  is_active: boolean;
}

function draftFrom(category: Category | null): CategoryDraft {
  return category
    ? {
        name: category.name,
        kind: category.kind,
        icon: category.icon,
        color: category.color,
        is_active: category.is_active,
      }
    : { name: "", kind: "expense", icon: defaultIcon, color: defaultColor, is_active: true };
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
  onSubmit: (draft: CategoryDraft) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState<CategoryDraft>(() => draftFrom(category));
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
    onSubmit({ ...draft, name: draft.name.trim() });
  };

  return (
    <Drawer
      title={category ? "Editar categoria" : "Nova categoria"}
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

        {category && (
          <div className="filter-field">
            <label htmlFor="category-active">Ativa</label>
            <Switch
              id="category-active"
              aria-label="Categoria ativa"
              checked={draft.is_active}
              onChange={(checked) => setDraft((current) => ({ ...current, is_active: checked }))}
              style={{ width: "fit-content" }}
            />
          </div>
        )}

        <div className="filter-field">
          <label htmlFor="category-icon">Ícone</label>
          <Select
            id="category-icon"
            value={draft.icon}
            options={iconOptions}
            showSearch
            optionFilterProp="value"
            onChange={(value: string) => setDraft((current) => ({ ...current, icon: value }))}
          />
        </div>

        <div className="filter-field">
          <label id="category-color-label">Cor</label>
          <div className="category-color-grid" role="radiogroup" aria-labelledby="category-color-label">
            {categoryColorPalette.map((hex) => {
              const selected = draft.color === hex;
              return (
                <button
                  key={hex}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  aria-label={`Cor ${hex}`}
                  className="category-color-swatch"
                  style={{ backgroundColor: hex }}
                  onClick={() => setDraft((current) => ({ ...current, color: hex }))}
                >
                  {selected && <CheckOutlined style={{ color: "#fff" }} aria-hidden="true" />}
                </button>
              );
            })}
          </div>
        </div>
      </Flex>
    </Drawer>
  );
}
