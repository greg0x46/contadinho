import { List } from "antd";

import type { ScenarioImpact } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

// ProjectionComposition lists each active scenario's own isolated impact —
// scenario_impacts[] already comes pre-computed from the API (one
// BuildSeries per scenario, isolated against Base), never recomputed here.
export function ProjectionComposition({ impacts }: { impacts: ScenarioImpact[] }) {
  if (impacts.length === 0) return null;
  return (
    <List
      header={<strong>Impacto por cenário</strong>}
      dataSource={impacts}
      renderItem={(impact) => (
        <List.Item className="timeline-category-impact-row">
          <span>{impact.scenario_name}</span>
          <strong>{formatBRL(impact.delta)}</strong>
        </List.Item>
      )}
    />
  );
}
