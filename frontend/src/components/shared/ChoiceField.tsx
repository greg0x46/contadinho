import { DownOutlined, SearchOutlined } from "@ant-design/icons";
import { Drawer, Input, Select, Spin, Tag, Typography } from "antd";
import { useState, type ReactNode } from "react";

import { useCompactScreen } from "./useCompactScreen";

export type ChoiceOption = {
  value: string;
  label: string;
  /** Leading glyph, e.g. the category icon. */
  icon?: ReactNode;
  /** Trailing badge, e.g. "Sugestão". */
  tag?: string;
};

/**
 * A single-choice field that follows the viewport: a searchable combobox
 * from `md` up, and a bottom sheet on a compact screen, where a dropdown
 * would fight the keyboard and the panel's own scroll.
 *
 * The API is the subset of antd Select the detail screens need, so a
 * caller never branches on the viewport itself.
 */
export function ChoiceField({
  id,
  label,
  value,
  options,
  placeholder = "Selecione",
  onChange,
  loading = false,
  disabled = false,
  emptyText = "Nenhuma opção disponível",
}: {
  id: string;
  /** Accessible name; also the sheet's title. */
  label: string;
  value: string | null;
  options: ChoiceOption[];
  placeholder?: string;
  onChange: (value: string) => void;
  loading?: boolean;
  disabled?: boolean;
  emptyText?: string;
}) {
  const compact = useCompactScreen();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [search, setSearch] = useState("");
  const selected = options.find((option) => option.value === value) ?? null;

  const renderOption = (option: ChoiceOption) => (
    <span className="choice-option">
      {option.icon && (
        <span className="choice-option-icon" aria-hidden="true">
          {option.icon}
        </span>
      )}
      <span className="choice-option-label">{option.label}</span>
      {option.tag && <Tag color="blue">{option.tag}</Tag>}
    </span>
  );

  if (!compact) {
    return (
      <Select
        id={id}
        aria-label={label}
        style={{ width: "100%" }}
        value={value ?? undefined}
        placeholder={placeholder}
        loading={loading}
        disabled={disabled}
        showSearch
        optionFilterProp="label"
        options={options.map((option) => ({ value: option.value, label: option.label, data: option }))}
        optionRender={(option) => renderOption(option.data.data)}
        notFoundContent={loading ? <Spin size="small" /> : emptyText}
        onChange={(next: string) => onChange(next)}
      />
    );
  }

  const normalized = search.trim().toLocaleLowerCase("pt-BR");
  const visible = normalized
    ? options.filter((option) => option.label.toLocaleLowerCase("pt-BR").includes(normalized))
    : options;

  const choose = (next: string) => {
    setSheetOpen(false);
    setSearch("");
    onChange(next);
  };

  return (
    <>
      <button
        type="button"
        id={id}
        className={`choice-field-trigger ${selected ? "" : "is-placeholder"}`}
        aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={sheetOpen}
        disabled={disabled || loading}
        onClick={() => setSheetOpen(true)}
      >
        <span className="choice-field-value">
          {selected ? renderOption(selected) : placeholder}
        </span>
        {loading ? <Spin size="small" /> : <DownOutlined aria-hidden="true" />}
      </button>
      <Drawer
        open={sheetOpen}
        onClose={() => setSheetOpen(false)}
        placement="bottom"
        height="72vh"
        title={label}
        className="choice-sheet"
        destroyOnHidden
      >
        <Input
          allowClear
          autoFocus
          prefix={<SearchOutlined aria-hidden="true" />}
          placeholder="Buscar"
          aria-label={`Buscar em ${label}`}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <ul className="choice-sheet-list" role="listbox" aria-label={label}>
          {visible.map((option) => (
            <li key={option.value}>
              <button
                type="button"
                role="option"
                aria-selected={option.value === value}
                className={`choice-sheet-option ${option.value === value ? "is-selected" : ""}`}
                onClick={() => choose(option.value)}
              >
                {renderOption(option)}
              </button>
            </li>
          ))}
        </ul>
        {visible.length === 0 && (
          <Typography.Text type="secondary" className="choice-sheet-empty">
            {emptyText}
          </Typography.Text>
        )}
      </Drawer>
    </>
  );
}
