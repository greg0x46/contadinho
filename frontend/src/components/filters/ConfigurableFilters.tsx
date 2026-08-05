import { FilterOutlined, SearchOutlined } from "@ant-design/icons";
import {
  Button,
  Checkbox,
  Drawer,
  Form,
  Input,
  InputNumber,
  Popover,
  Segmented,
  Select,
  Tag,
} from "antd";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

export type FilterFieldType =
  | "text"
  | "select"
  | "multiselect"
  | "date-range"
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

export interface DateRangePreset {
  value: string;
  label: string;
  range: () => [string, string];
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
  presets?: DateRangePreset[];
  hideChip?: boolean;
  formatActive?: (value: unknown, values: Values, option?: FilterOption) => string;
  /** Optional richer rendering (e.g. icon + color) for each dropdown option. */
  optionRender?: (option: FilterOption) => ReactNode;
}

type Props<Values extends object> = {
  values: Values;
  emptyValues: Values;
  config: FilterConfig<Values>[];
  onApply: (values: Values) => void;
  onClear: () => void;
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
  values,
  emptyValues,
  config,
  onApply,
  onClear,
}: Props<Values>) {
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [customDateOpen, setCustomDateOpen] = useState(false);
  const [customDates, setCustomDates] = useState<[string, string]>(["", ""]);
  const [quickValues, setQuickValues] = useState(values);
  const [draft, setDraft] = useState(values);
  const [error, setError] = useState<string | null>(null);
  const quickTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
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
        field.type === "date-range" &&
        (first === null || first === "") !== (second === null || second === "")
      ) {
        setError(`Preencha os dois campos de ${field.label.toLocaleLowerCase("pt-BR")}.`);
        return false;
      }
      if (
        typeof first === "string" &&
        first !== "" &&
        typeof second === "string" &&
        second !== "" &&
        (field.type === "value-range"
          ? compareDecimalStrings(first, second) > 0
          : first > second)
      ) {
        setError(
          field.type === "date-range"
            ? "A data inicial não pode ser posterior à data final."
            : "O valor mínimo não pode ser maior que o valor máximo.",
        );
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

    if (field.type === "date-range") {
      const selectedPreset = field.presets?.find((preset) => {
        const [from, to] = preset.range();
        return value === from && secondary === to;
      });
      const customContent = (
        <div className="filter-date-panel">
          <label htmlFor={`${id}-from`}>Data inicial</label>
          <Input
            id={`${id}-from`}
            type="date"
            value={customDates[0]}
            onChange={(event) =>
              setCustomDates((current) => [event.target.value, current[1]])
            }
          />
          <label htmlFor={`${id}-to`}>Data final</label>
          <Input
            id={`${id}-to`}
            type="date"
            value={customDates[1]}
            onChange={(event) =>
              setCustomDates((current) => [current[0], event.target.value])
            }
          />
          {error && <p role="alert">{error}</p>}
          <Button
            type="primary"
            onClick={() => {
              const candidate = update(
                quick ? quickValues : draft,
                field,
                customDates[0] || null,
                customDates[1] || null,
              );
              if (validate(candidate)) {
                if (quick) onApply(candidate);
                else setDraft(candidate);
                setCustomDateOpen(false);
              }
            }}
          >
            Confirmar período
          </Button>
        </div>
      );
      return (
        <div className="filter-field filter-field-period" key={field.key}>
          <label htmlFor={id}>{field.label}</label>
          <div className="filter-period-control">
            <Select
              id={id}
              aria-label={field.label}
              value={selectedPreset?.value ?? "custom"}
              options={[
                ...(field.presets?.map(({ value: presetValue, label }) => ({
                  value: presetValue,
                  label,
                })) ?? []),
                { value: "custom", label: "Personalizado" },
              ]}
              onChange={(presetValue) => {
                if (presetValue === "custom") {
                  setCustomDates([
                    typeof value === "string" ? value : "",
                    typeof secondary === "string" ? secondary : "",
                  ]);
                  setCustomDateOpen(true);
                  return;
                }
                const preset = field.presets?.find((item) => item.value === presetValue);
                if (preset) commit(...preset.range());
              }}
            />
            {(!selectedPreset || customDateOpen) && (
              <Popover
                title="Período personalizado"
                trigger="click"
                open={customDateOpen}
                onOpenChange={(open) => {
                  if (open) {
                    setCustomDates([
                      typeof value === "string" ? value : "",
                      typeof secondary === "string" ? secondary : "",
                    ]);
                  }
                  setCustomDateOpen(open);
                }}
                content={customContent}
              >
                <Button
                  className="filter-custom-period-trigger"
                  aria-label="Editar período personalizado"
                >
                  Editar
                </Button>
              </Popover>
            )}
          </div>
        </div>
      );
    }

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
      <div className="filter-field filter-field-search" key={field.key}>
        <label htmlFor={id}>{field.label}</label>
        <Input
          id={id}
          aria-label={field.label}
          allowClear
          prefix={<SearchOutlined aria-hidden="true" />}
          placeholder={field.placeholder}
          value={typeof value === "string" ? value : ""}
          onChange={(event) => commit(event.target.value || null)}
        />
      </div>
    );
  };

  return (
    <section className="configurable-filters" aria-label="Filtros de transações">
      <div className="filter-main-bar">
        {mainFields.map((field) => renderField(field, quickValues, true))}
        <Button
          className="advanced-filter-button"
          icon={<FilterOutlined aria-hidden="true" />}
          onClick={() => setAdvancedOpen(true)}
        >
          Filtros{advancedCount ? ` ${advancedCount}` : ""}
        </Button>
      </div>

      {chips.length > 0 && (
        <div className="active-filter-list" aria-label="Filtros ativos">
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
      )}

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
            <div className="filter-mobile-main">
              {mainFields
                .filter((field) => field.type === "select" || field.type === "multiselect")
                .map((field) => renderField(field, draft, false))}
            </div>
            {advancedFields.map((field) => renderField(field, draft, false))}
          </div>
          {error && <p role="alert">{error}</p>}
        </Form>
      </Drawer>
    </section>
  );
}
