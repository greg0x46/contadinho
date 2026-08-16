import { Tag } from "antd";

import type { CertaintyTier } from "../../api/contracts";
import { certaintyTierLabel } from "../../presentation/timelineLabels";

const tierColor: Record<CertaintyTier, string> = {
  realizado: "success",
  confirmado: "processing",
  projetado: "default",
  hipotetico: "purple",
};

export function EntryOriginBadge({ tier }: { tier: CertaintyTier }) {
  return <Tag color={tierColor[tier]}>{certaintyTierLabel[tier]}</Tag>;
}
