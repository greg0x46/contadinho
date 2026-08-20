import { Card } from "antd";
import type { CSSProperties, ReactNode } from "react";

interface WidgetCardProps {
  icon: ReactNode;
  title: string;
  extra?: ReactNode;
  style?: CSSProperties;
  children: ReactNode;
}

/**
 * Shared shell for the "icon + title" summary cards used across the Home
 * dashboard and the accounts/payables/investments domain summaries — same
 * Card, same title layout, so the four+ near-identical copies of this
 * markup collapse into one place.
 */
export function WidgetCard({ icon, title, extra, style, children }: WidgetCardProps) {
  return (
    <Card
      className="dashboard-widget"
      style={style}
      title={
        <span className="dashboard-widget-title">
          {icon}
          {title}
        </span>
      }
      extra={extra}
    >
      {children}
    </Card>
  );
}
