import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Button, Flex, Switch, Tag } from "antd";

import type { Category } from "../../api/contracts";
import { categoryKindColor, categoryKindLabel, renderCategoryIcon } from "../../presentation/categoryLabels";

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
    {
      title: "Nome",
      dataIndex: "name",
      render: (_, category) => (
        <Flex align="center" gap="small">
          <span className="category-badge" style={{ backgroundColor: category.color }} aria-hidden="true">
            {renderCategoryIcon(category.icon)}
          </span>
          <span>{category.name}</span>
        </Flex>
      ),
    },
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
        <span onClick={(event) => event.stopPropagation()}>
          <Switch
            aria-label={`${category.is_active ? "Desativar" : "Ativar"} categoria ${category.name}`}
            checked={category.is_active}
            loading={togglingCategoryId === category.id}
            onChange={(checked) => onToggle(category, checked)}
          />
        </span>
      ),
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, category) => (
        <span onClick={(event) => event.stopPropagation()}>
          <Button type="link" onClick={() => onRename(category)}>
            Editar
          </Button>
        </span>
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
      onRow={(category) => ({
        onClick: () => onRename(category),
        style: { cursor: "pointer" },
      })}
    />
  );
}
