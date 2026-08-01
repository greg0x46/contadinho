import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { Tag } from "antd";
import type { SyncStatus } from "../api/contracts";
import { getStatusMetadata } from "../presentation/syncStatus";

export function SyncStatusBadge({ status }: { status: SyncStatus }) {
  const metadata = getStatusMetadata(status);
  const color = {
    progress: "processing",
    success: "success",
    warning: "warning",
    danger: "error",
  }[metadata.tone];
  const icon = {
    progress: <SyncOutlined aria-hidden />,
    success: <CheckCircleOutlined aria-hidden />,
    warning: <WarningOutlined aria-hidden />,
    danger: <CloseCircleOutlined aria-hidden />,
  }[metadata.tone];

  return (
    <Tag color={color} icon={icon}>
      {metadata.label}
    </Tag>
  );
}
