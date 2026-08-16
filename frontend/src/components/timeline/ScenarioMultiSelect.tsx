import { PlusOutlined } from "@ant-design/icons";
import { Select, Tag } from "antd";

import { useScenarios } from "../../hooks/useScenarios";

// ScenarioMultiSelect drives the URL's scenario_ids field (owned by the
// page, serialized the same way every other filter is via filterUrl.ts) —
// adding or removing a scenario here changes the URL, which changes
// useTimeline's queryKey, which already triggers a refetch on its own. No
// extra recalculation logic lives here.
export function ScenarioMultiSelect({
  activeIds,
  onChange,
}: {
  activeIds: string[];
  onChange: (ids: string[]) => void;
}) {
  const scenarios = useScenarios({ kind: "standalone" });
  const options = scenarios.scenarios.map((s) => ({ value: s.id, label: s.name }));

  return (
    <div className="timeline-scenario-select">
      <Select
        mode="multiple"
        aria-label="Cenários ativos"
        placeholder={
          <span>
            <PlusOutlined aria-hidden="true" /> Adicionar cenário
          </span>
        }
        style={{ minWidth: 260 }}
        value={activeIds}
        options={options}
        loading={scenarios.isLoading}
        tagRender={({ label, onClose }) => (
          <Tag closable onClose={onClose} style={{ marginInlineEnd: 4 }}>
            {label}
          </Tag>
        )}
        onChange={onChange}
      />
      {activeIds.length > 0 && (
        <button type="button" className="timeline-scenario-clear" onClick={() => onChange([])}>
          Remover todos
        </button>
      )}
    </div>
  );
}
