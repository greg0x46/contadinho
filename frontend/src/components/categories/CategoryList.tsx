import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Switch, Tag } from "antd";

import type { Category } from "../../api/contracts";
import { categoryKindColor, categoryKindLabel } from "../../presentation/categoryLabels";

export function CategoryList({
  categories,
  isLoading,
  togglingCategoryId,
  onRename,
  onToggle,
}: {
  categories: Category[];
  isLoading: boolean;
  togglingCategoryId: string | null;
  onRename: (category: Category) => void;
  onToggle: (category: Category, isActive: boolean) => void;
}) {
  const columns: ProColumns<Category>[] = [
    { title: "Nome", dataIndex: "name" },
    {
      title: "Tipo",
      dataIndex: "kind",
      render: (_, category) => (
        <Tag color={categoryKindColor[category.kind]}>{categoryKindLabel[category.kind]}</Tag>
      ),
      filters: (Object.keys(categoryKindLabel) as Category["kind"][]).map((kind) => ({
        text: categoryKindLabel[kind],
        value: kind,
      })),
      onFilter: (value, category) => category.kind === value,
    },
    {
      title: "Ativa",
      dataIndex: "is_active",
      render: (_, category) => (
        <Switch
          aria-label={`${category.is_active ? "Desativar" : "Ativar"} categoria ${category.name}`}
          checked={category.is_active}
          loading={togglingCategoryId === category.id}
          onChange={(checked) => onToggle(category, checked)}
        />
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, category) => (
        <Button type="link" onClick={() => onRename(category)}>
          Renomear
        </Button>
      ),
    },
  ];

  return (
    <ProTable<Category>
      aria-label="Categorias"
      columns={columns}
      dataSource={categories}
      loading={isLoading}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      locale={{ emptyText: "Nenhuma categoria encontrada." }}
    />
  );
}
