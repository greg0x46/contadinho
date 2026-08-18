import { LeftOutlined, RightOutlined } from "@ant-design/icons";
import { Button, DatePicker, Flex } from "antd";
import type { Dayjs } from "dayjs";

export function TimeNavigator({
  month,
  onChange,
}: {
  month: Dayjs;
  onChange: (month: Dayjs) => void;
}) {
  return (
    <Flex align="center" gap="small" className="timeline-navigator">
      <Button
        aria-label="Mês anterior"
        icon={<LeftOutlined aria-hidden="true" />}
        onClick={() => onChange(month.subtract(1, "month"))}
      />
      <DatePicker
        aria-label="Mês e ano"
        picker="month"
        format="MMMM [de] YYYY"
        value={month}
        allowClear={false}
        onChange={(value) => value && onChange(value)}
      />
      <Button
        aria-label="Próximo mês"
        icon={<RightOutlined aria-hidden="true" />}
        onClick={() => onChange(month.add(1, "month"))}
      />
    </Flex>
  );
}
