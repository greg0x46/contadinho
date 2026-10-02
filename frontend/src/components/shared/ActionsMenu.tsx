import { EllipsisOutlined } from "@ant-design/icons";
import { Button, Dropdown, Popconfirm } from "antd";
import { useState } from "react";

export type ActionsMenuItem = {
  key: string;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  danger?: boolean;
  /** Destructive items ask first, anchored to the "···" they came from. */
  confirm?: { title: string; description?: string; okText?: string };
};

/**
 * Secondary actions behind a single "···". Items that are not available are
 * left out by the caller instead of shown as static text; when nothing is
 * left the trigger itself is not rendered.
 */
export function ActionsMenu({
  items,
  label,
  size = "small",
  className,
}: {
  items: ActionsMenuItem[];
  /** Accessible name of the trigger, e.g. "Ações de CDB Nu". */
  label: string;
  size?: "small" | "middle";
  className?: string;
}) {
  const [confirming, setConfirming] = useState<ActionsMenuItem | null>(null);
  if (items.length === 0) return null;

  const trigger = (
    <Dropdown
      trigger={["click"]}
      placement="bottomRight"
      menu={{
        items: items.map(({ key, label: itemLabel, disabled, danger }) => ({ key, label: itemLabel, disabled, danger })),
        onClick: ({ key }) => {
          const item = items.find((candidate) => candidate.key === key);
          if (!item) return;
          if (item.confirm) setConfirming(item);
          else item.onClick();
        },
      }}
    >
      <Button
        type="text"
        size={size}
        className={["actions-menu-trigger", className].filter(Boolean).join(" ")}
        icon={<EllipsisOutlined aria-hidden="true" />}
        aria-label={label}
      />
    </Dropdown>
  );

  return (
    <Popconfirm
      open={confirming !== null}
      title={confirming?.confirm?.title}
      description={confirming?.confirm?.description}
      okText={confirming?.confirm?.okText ?? "Remover"}
      cancelText="Cancelar"
      okButtonProps={{ danger: true }}
      placement="bottomRight"
      overlayClassName="actions-menu-confirm"
      onConfirm={() => {
        confirming?.onClick();
        setConfirming(null);
      }}
      onCancel={() => setConfirming(null)}
      onOpenChange={(open) => {
        if (!open) setConfirming(null);
      }}
    >
      {trigger}
    </Popconfirm>
  );
}
