import { EllipsisOutlined } from "@ant-design/icons";
import { Button, Dropdown } from "antd";
import type { ItemType } from "antd/es/menu/interface";

import type { TransactionInclusionState, TransactionItem } from "../../api/contracts";

/**
 * The row's secondary actions behind a single "···": what used to be a
 * permanent "Ignorar" link on every line. Only actions the row can already
 * perform are listed — deep links into the panel's sub-screens stay in the
 * panel.
 */
export function TransactionActions({
  item,
  onSelect,
  onInclusion,
  pending = false,
}: {
  item: TransactionItem;
  onSelect: (id: string) => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  pending?: boolean;
}) {
  const ignored = item.inclusion.state === "ignored";
  const name = item.description ?? "transação sem descrição";
  const items: ItemType[] = [
    { key: "details", label: "Ver detalhes", onClick: () => onSelect(item.id) },
    {
      key: "inclusion",
      label: ignored ? "Restaurar" : "Ignorar",
      disabled: !onInclusion || pending,
      onClick: () => onInclusion?.(item.id, ignored ? "considered" : "ignored"),
    },
  ];
  return (
    // Sits above the row's stretched hit area (see TransactionRow).
    <span className="transaction-actions">
      <Dropdown menu={{ items }} trigger={["click"]} placement="bottomRight">
        <Button
          type="text"
          size="small"
          className="transaction-actions-trigger"
          loading={pending}
          icon={<EllipsisOutlined aria-hidden="true" />}
          aria-label={`Ações de ${name}`}
        />
      </Dropdown>
    </span>
  );
}
