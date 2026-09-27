import { CloseOutlined, PlusOutlined } from "@ant-design/icons";
import { Button, Drawer, Dropdown } from "antd";
import type { ItemType } from "antd/es/menu/interface";
import type { ReactNode } from "react";
import { useState } from "react";

import { PanelNavRow } from "../shared/PanelStack";
import { useCompactScreen } from "../shared/useCompactScreen";

export interface CreateActionOption {
  key: string;
  label: string;
  /** Quiet, one-line context under the label — what the option is for, not how to use it. */
  description?: string;
  /** A small mark for the option's kind, shown on the phone sheet only. */
  icon?: ReactNode;
  onClick: () => void;
  disabled?: boolean;
}

interface CreateActionMenuProps {
  /** The trigger's own label, e.g. "Nova pendência" — one thing that groups the options. */
  label: string;
  options: CreateActionOption[];
}

/**
 * A single compact trigger for several equally-weighted creation options —
 * the alternative to stacking one big button per option (which manufactures
 * a hierarchy between choices that don't have one). From `md` up it opens a
 * dropdown menu next to the trigger; on a phone it opens a bottom sheet,
 * the same split `FilterPanel` uses for its own drawer-vs-sheet choice.
 *
 * The trigger stays a single compact action regardless of viewport — it
 * replaces what used to be N full-width stacked buttons, so it deliberately
 * opts out of the page header's default "actions fill the width" rule (see
 * `.create-action-trigger` in layout.css).
 */
export function CreateActionMenu({ label, options }: CreateActionMenuProps) {
  const compact = useCompactScreen();
  const [open, setOpen] = useState(false);

  const trigger = (
    <Button
      type="primary"
      className="create-action-trigger"
      icon={<PlusOutlined aria-hidden="true" />}
      onClick={compact ? () => setOpen(true) : undefined}
    >
      {label}
    </Button>
  );

  if (!compact) {
    const items: ItemType[] = options.map((option) => ({
      key: option.key,
      disabled: option.disabled,
      label: (
        <span className="create-action-menu-option">
          <span className="create-action-menu-option-label">{option.label}</span>
          {option.description && (
            <span className="create-action-menu-option-description">{option.description}</span>
          )}
        </span>
      ),
      onClick: option.onClick,
    }));
    return (
      <Dropdown menu={{ items }} trigger={["click"]} placement="bottomRight">
        {trigger}
      </Dropdown>
    );
  }

  return (
    <>
      {trigger}
      <Drawer
        open={open}
        onClose={() => setOpen(false)}
        placement="bottom"
        height="auto"
        closable={false}
        destroyOnHidden
        className="create-action-sheet"
        title={
          <>
            <div className="create-action-sheet-handle" aria-hidden="true" />
            <div className="create-action-sheet-header">
              <h2 className="create-action-sheet-title">{label}</h2>
              <Button
                type="text"
                className="create-action-sheet-close"
                icon={<CloseOutlined aria-hidden="true" />}
                aria-label={`Fechar ${label.toLocaleLowerCase("pt-BR")}`}
                onClick={() => setOpen(false)}
              />
            </div>
          </>
        }
      >
        <ul className="create-action-sheet-list">
          {options.map((option) => (
            <li key={option.key}>
              <PanelNavRow
                icon={option.icon}
                label={option.label}
                hint={option.description}
                disabled={option.disabled}
                onClick={() => {
                  setOpen(false);
                  option.onClick();
                }}
              />
            </li>
          ))}
        </ul>
      </Drawer>
    </>
  );
}
