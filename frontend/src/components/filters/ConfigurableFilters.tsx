import {
  Button,
  Checkbox,
  Drawer,
  Form,
  InputNumber,
  Segmented,
  Select,
  Tag,
} from "antd";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { FilterButton } from "../layout/FilterButton";
import { ListToolbar } from "../layout/ListToolbar";
import { SearchField } from "../layout/SearchField";

export type FilterFieldType =
  | "text"
  | "select"
  | "multiselect"
  | "value-range"
  | "segmented"
  | "boolean";

export interface FilterOption {
  value: string;
  label: string;
  disabled?: boolean;
  /** Purely additive — only consumers that opt in via FilterConfig.optionRender read these. */
  icon?: string;
  color?: string;
}

export interface FilterConfig<Values extends object> {
  key: Extract<keyof Values, string>;
  secondaryKey?: Extract<keyof Values, string>;
  label: string;
  type: FilterFieldType;
  placement: "main" | "advanced";
  placeholder?: string;
  debounceMs?: number;
  options?: FilterOption[] | ((values: Values) => FilterOption[]);
  hideChip?: boolean;
  formatActive?: (value: unknown, values: Values, option?: FilterOption) => string;
  /** Optional richer rendering (e.g. icon + color) for each dropdown option. */
  optionRender?: (option: FilterOption) => ReactNode;
}

type Props<Values extends object> = {
  /** Names the toolbar for assistive tech: "Filtros de transações". */
  label: string;
  values: Values;
  emptyValues: Values;
  config: FilterConfig<Values>[];
  onApply: (values: Values) => void;
  onClear: () => void;
  /** The shaping side of the toolbar (group by, sort), owned by the caller. */
  end?: ReactNode;
};

const read = <Values extends object>(
  values: Values,
  key: Extract<keyof Values, string>,
): unknown => (values as Record<string, unknown>)[key];

const write = <Values extends object>(
  values: Values,
  key: Extract<keyof Values, string>,
  value: unknown,
): Values => ({ ...values, [key]: value });

function fieldOptions<Values extends object>(
  field: FilterConfig<Values>,
  values: Values,
): FilterOption[] {
  if (typeof field.options === "function") return field.options(values);
  return field.options ?? [];
}

function fieldIsActive<Values extends object>(
  field: FilterConfig<Values>,
  values: Values,
): boolean {
  const value = read(values, field.key);
  const secondary = field.secondaryKey ? read(values, field.secondaryKey) : null;
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === "boolean") return value;
  return value !== null && value !== undefined && value !== "" ||
    secondary !== null && secondary !== undefined && secondary !== "";
}

function replaceField<Values extends object>(
  values: Values,
  field: FilterConfig<Values>,
  emptyValues: Values,
): Values {
  let next = write(values, field.key, read(emptyValues, field.key));
  if (field.secondaryKey) {
    next = write(next, field.secondaryKey, read(emptyValues, field.secondaryKey));
  }
  return next;
}

function compareDecimalStrings(left: string, right: string): number {
  const [leftInteger = "0", leftFraction = ""] = left.split(".");
  const [rightInteger = "0", rightFraction = ""] = right.split(".");
  const normalizedLeft = leftInteger.replace(/^0+(?=\d)/, "");
  const normalizedRight = rightInteger.replace(/^0+(?=\d)/, "");
  if (normalizedLeft.length !== normalizedRight.length) {
    return normalizedLeft.length - normalizedRight.length;
  }
  if (normalizedLeft !== normalizedRight) {
    return normalizedLeft.localeCompare(normalizedRight);
  }
  const scale = Math.max(leftFraction.length, rightFraction.length);
  return leftFraction.padEnd(scale, "0").localeCompare(rightFraction.padEnd(scale, "0"));
}

export function ConfigurableFilters<Values extends object>({
  label,
  values,
  emptyValues,
  config,
  onApply,
  onClear,
  end,
}: Props<Values>) {
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [quickValues, setQuickValues] = useState(values);
  const [draft, setDraft] = useState(values);
  const [error, setError] = useState<string | null>(null);
  const quickTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (quickTimer.current) clearTimeout(quickTimer.current);
    setQuickValues(values);
    setDraft(values);
  }, [values]);
  useEffect(
    () => () => {
      if (quickTimer.current) clearTimeout(quickTimer.current);
    },
    [],
  );

  const mainFields = config.filter((field) => field.placement === "main");
  const advancedFields = config.filter((field) => field.placement === "advanced");
  const advancedCount = advancedFields.filter((field) => fieldIsActive(field, values)).length;
  const chips = config.filter(
    (field) => !field.hideChip && fieldIsActive(field, values),
  );

  const optionsByKey = useMemo(
    () =>
      Object.fromEntries(
        config.map((field) => [field.key, fieldOptions(field, values)]),
      ) as Record<string, FilterOption[]>,
    [config, values],
  );

  const update = (
    source: Values,
    field: FilterConfig<Values>,
    value: unknown,
    secondary?: unknown,
  ) => {
    let next = write(source, field.key, value);
    if (field.secondaryKey && secondary !== undefined) {
      next = write(next, field.secondaryKey, secondary);
    }
    return next;
  };

  const validate = (candidate: Values): boolean => {
    for (const field of config) {
      if (!field.secondaryKey) continue;
      const first = read(candidate, field.key);
      const second = read(candidate, field.secondaryKey);
      if (
        typeof first === "string" &&
        first !== "" &&
        typeof second === "string" &&
        second !== "" &&
        compareDecimalStrings(first, second) > 0
      ) {
        setError("O valor mínimo não pode ser maior que o valor máximo.");
        return false;
      }
    }
    setError(null);
    return true;
  };

  const applyQuick = (
    field: FilterConfig<Values>,
    value: unknown,
    secondary?: unknown,
  ) => {
    const next = update(quickValues, field, value, secondary);
    setQuickValues(next);
    if (!validate(next)) return;
    if (quickTimer.current) clearTimeout(quickTimer.current);
    if (field.debounceMs) {
      quickTimer.current = setTimeout(() => onApply(next), field.debounceMs);
    } else {
      onApply(next);
    }
  };

  const renderField = (
    field: FilterConfig<Values>,
    source: Values,
    quick: boolean,
  ) => {
    const value = read(source, field.key);
    const secondary = field.secondaryKey ? read(source, field.secondaryKey) : null;
    const commit = (nextValue: unknown, nextSecondary?: unknown) => {
      if (quick) applyQuick(field, nextValue, nextSecondary);
      else setDraft((current) => update(current, field, nextValue, nextSecondary));
    };
    const options = fieldOptions(field, source);
    const id = `filter-${field.key}`;

    if (field.type === "value-range") {
      return (
        <Form.Item key={field.key} label={field.label}>
          <div className="filter-value-range">
            <InputNumber
              id={`${id}-min`}
              aria-label="Valor mínimo"
              stringMode
              min="0"
              controls={false}
              placeholder="Mínimo"
              value={typeof value === "string" ? value : null}
              onChange={(next) => commit(next, secondary)}
            />
            <span aria-hidden="true">até</span>
            <InputNumber
              id={`${id}-max`}
              aria-label="Valor máximo"
              stringMode
              min="0"
              controls={false}
              placeholder="Máximo"
              value={typeof secondary === "string" ? secondary : null}
              onChange={(next) => commit(value, next)}
            />
          </div>
        </Form.Item>
      );
    }

    if (field.type === "boolean") {
      return (
        <Form.Item key={field.key}>
          <Checkbox checked={value === true} onChange={(event) => commit(event.target.checked)}>
            {field.label}
          </Checkbox>
        </Form.Item>
      );
    }

    if (field.type === "segmented") {
      return (
        <Form.Item key={field.key} label={field.label}>
          <Segmented
            block
            value={typeof value === "string" && value ? value : options[0]?.value}
            options={options}
            onChange={commit}
          />
        </Form.Item>
      );
    }

    if (field.type === "select" || field.type === "multiselect") {
      return (
        <div className="filter-field" key={field.key}>
          <label htmlFor={id}>{field.label}</label>
          <Select
            id={id}
            aria-label={field.label}
            mode={field.type === "multiselect" ? "multiple" : undefined}
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder={field.placeholder}
            value={value === null ? undefined : value as string | string[]}
            options={options}
            optionRender={field.optionRender ? (option) => field.optionRender!(option.data as FilterOption) : undefined}
            notFoundContent="Nenhuma opção disponível"
            onChange={(next) => commit(next ?? null)}
          />
        </div>
      );
    }

    return (
      <SearchField
        key={field.key}
        id={id}
        label={field.label}
        placeholder={field.placeholder}
        value={typeof value === "string" ? value : ""}
        onChange={(next) => commit(next || null)}
      />
    );
  };

  const chipList = chips.length > 0 && (
    <div className="list-toolbar-chips" aria-label="Filtros ativos">
      {chips.map((field) => {
        const value = read(values, field.key);
        const option = optionsByKey[field.key]?.find((item) => item.value === value);
        const shown = field.formatActive
          ? field.formatActive(value, values, option)
          : option?.label ?? `${field.label}: ${String(value)}`;
        return (
          <Tag
            key={field.key}
            closable
            onClose={(event) => {
              event.preventDefault();
              onApply(replaceField(values, field, emptyValues));
            }}
            closeIcon={<span aria-label={`Remover ${shown}`}>×</span>}
          >
            {shown}
          </Tag>
        );
      })}
      <Button type="link" size="small" onClick={onClear}>
        Limpar filtros
      </Button>
    </div>
  );

  return (
    <>
      <ListToolbar
        label={label}
        start={
          <>
            {mainFields.map((field) => renderField(field, quickValues, true))}
            <FilterButton activeCount={advancedCount} onClick={() => setAdvancedOpen(true)} />
          </>
        }
        end={end}
        chips={chipList || undefined}
      />
      <Drawer
        title="Filtros avançados"
        placement="right"
        width={400}
        open={advancedOpen}
        onClose={() => {
          setDraft(values);
          setError(null);
          setAdvancedOpen(false);
        }}
        footer={
          <div className="advanced-filter-actions">
            <Button
              onClick={() => {
                let next = draft;
                for (const field of advancedFields) {
                  next = replaceField(next, field, emptyValues);
                }
                setDraft(next);
                setError(null);
              }}
            >
              Limpar filtros
            </Button>
            <Button
              type="primary"
              onClick={() => {
                if (!validate(draft)) return;
                onApply(draft);
                setAdvancedOpen(false);
              }}
            >
              Aplicar
            </Button>
          </div>
        }
      >
        <Form layout="vertical">
          <div className="advanced-filter-fields">
            {advancedFields.map((field) => renderField(field, draft, false))}
          </div>
          {error && <p role="alert">{error}</p>}
        </Form>
      </Drawer>
    </>
  );
}
