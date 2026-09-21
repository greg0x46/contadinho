import type {
  TransactionGroup as Group,
  TransactionInclusionState,
  TransactionItem,
} from "../../api/contracts";
import { TransactionGroupHeader } from "./TransactionGroupHeader";
import { TransactionRow } from "./TransactionRow";

/** One period of the transactions list: its header and its flat rows. */
export function TransactionGroup({
  group,
  items,
  selectedId = null,
  onSelect,
  onInclusion,
  pendingTransactionId,
}: {
  group: Group;
  items: TransactionItem[];
  /** The transaction whose panel is open, so its row reads as selected. */
  selectedId?: string | null;
  onSelect?: (id: string) => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  pendingTransactionId?: string | null;
}) {
  return (
    <section
      className={`transaction-group ${group.kind === "none" ? "transaction-group-flat" : ""}`}
      aria-labelledby={group.kind === "none" ? undefined : `group-${group.key}`}
    >
      {group.kind !== "none" && <TransactionGroupHeader group={group} />}
      <div className="transaction-list">
        {items.map((item) => (
          <TransactionRow
            key={item.id}
            item={item}
            selected={item.id === selectedId}
            onSelect={onSelect ?? (() => undefined)}
            onInclusion={onInclusion}
            inclusionPending={pendingTransactionId === item.id}
          />
        ))}
      </div>
    </section>
  );
}
