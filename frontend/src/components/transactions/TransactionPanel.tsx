import { DeleteOutlined, EditOutlined, MoreOutlined } from "@ant-design/icons";
import { useQueryClient } from "@tanstack/react-query";
import { Button, Dropdown, Modal } from "antd";
import { useEffect, useState } from "react";

import type {
  Account,
  Category,
  ManualTransactionWrite,
  RecurringCommitmentWrite,
  TransactionInclusionState,
  TransactionItem,
} from "../../api/contracts";
import { useRecurringCommitments } from "../../hooks/useRecurringCommitments";
import { transactionReconciliationQueryKey } from "../../hooks/useTransactionReconciliation";
import { recurringCommitmentPrefill } from "../../presentation/transactionDetail";
import { InvestmentLinkNewOperationScreen, InvestmentLinkScreen } from "../investments/InvestmentLinkScreen";
import { RecurringCommitmentFields } from "../recurringCommitments/RecurringCommitmentForm";
import { PanelFooter, PanelSection, PanelStack } from "../shared/PanelStack";
import { useScreenStack } from "../shared/useScreenStack";
import { ManualTransactionFields } from "./ManualTransactionForm";
import { TransactionDetailsScreen, TransactionTechnicalScreen } from "./TransactionInfoScreens";
import { TransactionOverviewScreen } from "./TransactionOverviewScreen";
import { TransactionReconcileScreen } from "./TransactionReconcileScreen";

/**
 * Every place the panel can show. Each is a whole screen that replaces the
 * previous one inside the same container — never a layer on top of it.
 */
export type TransactionScreen =
  | { kind: "overview" }
  | { kind: "details" }
  | { kind: "technical" }
  | { kind: "recurrence" }
  | { kind: "reconcile" }
  | { kind: "investment" }
  | { kind: "investment-new"; amount: string | null }
  | { kind: "edit-manual" };

const screenTitle: Record<TransactionScreen["kind"], string> = {
  overview: "Transação",
  details: "Detalhes da transação",
  technical: "Informações técnicas",
  recurrence: "Criar recorrência",
  reconcile: "Conciliar com recorrência",
  investment: "Vincular a investimento",
  "investment-new": "Nova movimentação",
  "edit-manual": "Editar lançamento",
};

const root: TransactionScreen = { kind: "overview" };

export type TransactionPanelProps = {
  item: TransactionItem | null;
  categories?: Category[];
  accounts?: Account[];
  onClose: () => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  inclusionPending?: boolean;
  onCategory?: (id: string, categoryId: string) => void;
  categoryPending?: boolean;
  /** Rejects with an Error whose message is ready to show. */
  onSaveManual?: (id: string, write: ManualTransactionWrite) => Promise<void>;
  saveManualPending?: boolean;
  onDeleteManual?: (id: string) => void;
  deleteManualPending?: boolean;
  deleteManualError?: string | null;
};

/**
 * The transaction's own workspace: a full-screen page on a phone, a side
 * panel on a desktop, and the same stack of screens inside either. The
 * stack is keyed by the transaction, so picking another line in the list
 * always lands on its overview.
 */
export function TransactionPanel(props: TransactionPanelProps) {
  // The last selected line stays rendered while `item` is null so the
  // container can animate closed instead of vanishing.
  const [shown, setShown] = useState<TransactionItem | null>(props.item);
  if (props.item !== null && props.item !== shown) setShown(props.item);
  if (shown === null) return null;
  return <TransactionStack key={shown.id} {...props} item={shown} open={props.item !== null} />;
}

function TransactionStack({
  item,
  open,
  categories = [],
  accounts = [],
  onClose,
  onInclusion,
  inclusionPending = false,
  onCategory,
  categoryPending = false,
  onSaveManual,
  saveManualPending = false,
  onDeleteManual,
  deleteManualPending = false,
  deleteManualError = null,
}: TransactionPanelProps & { item: TransactionItem; open: boolean }) {
  const stack = useScreenStack<TransactionScreen>(root);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const current = stack.current;
  // The stack outlives the container's content (which the Drawer destroys
  // on close), so reopening the same line must start from the overview.
  const { reset } = stack;
  useEffect(() => {
    if (open) reset();
  }, [open, reset]);
  const isManual = item.origin === "manual";

  const menu = isManual
    ? {
        items: [
          { key: "edit", icon: <EditOutlined />, label: "Editar lançamento", disabled: !onSaveManual },
          { key: "delete", icon: <DeleteOutlined />, label: "Excluir lançamento", danger: true, disabled: !onDeleteManual },
        ],
        onClick: ({ key }: { key: string }) => {
          if (key === "edit") stack.push({ kind: "edit-manual" });
          if (key === "delete") setConfirmDelete(true);
        },
      }
    : null;

  return (
    <PanelStack
      open={open}
      onClose={onClose}
      title={screenTitle[current.kind]}
      onBack={stack.depth > 1 ? stack.pop : undefined}
      extra={
        menu && current.kind === "overview" ? (
          <Dropdown menu={menu} trigger={["click"]} placement="bottomRight">
            <Button type="text" icon={<MoreOutlined />} aria-label="Mais ações" />
          </Dropdown>
        ) : null
      }
    >
      {current.kind === "overview" && (
        <TransactionOverviewScreen
          item={item}
          categories={categories}
          onCategory={onCategory}
          categoryPending={categoryPending}
          onInclusion={onInclusion}
          inclusionPending={inclusionPending}
          deleteError={deleteManualError}
          navigate={stack.push}
        />
      )}
      {current.kind === "details" && <TransactionDetailsScreen item={item} />}
      {current.kind === "technical" && <TransactionTechnicalScreen item={item} />}
      {current.kind === "recurrence" && (
        <CreateRecurrenceScreen item={item} categories={categories} onDone={stack.reset} />
      )}
      {current.kind === "reconcile" && <TransactionReconcileScreen item={item} onDone={stack.reset} />}
      {current.kind === "investment" && (
        <InvestmentLinkScreen
          transaction={item}
          onNewOperation={(amount) => stack.push({ kind: "investment-new", amount })}
          onDone={stack.reset}
        />
      )}
      {current.kind === "investment-new" && (
        <InvestmentLinkNewOperationScreen transaction={item} amount={current.amount} onDone={stack.reset} />
      )}
      {current.kind === "edit-manual" && onSaveManual && (
        <EditManualScreen
          item={item}
          accounts={accounts}
          categories={categories}
          submitting={saveManualPending}
          onSave={onSaveManual}
          onDone={stack.reset}
        />
      )}

      <Modal
        open={confirmDelete}
        title="Excluir lançamento manual"
        okText="Excluir"
        cancelText="Cancelar"
        okButtonProps={{ danger: true, loading: deleteManualPending }}
        onOk={() => {
          onDeleteManual?.(item.id);
          setConfirmDelete(false);
        }}
        onCancel={() => setConfirmDelete(false)}
      >
        Esta ação não pode ser desfeita.
      </Modal>
    </PanelStack>
  );
}

const recurrenceFormId = "transaction-recurrence-form";

/** "Criar recorrência" from a transaction: the commitment form seeded with the line's figures. */
function CreateRecurrenceScreen({
  item,
  categories,
  onDone,
}: {
  item: TransactionItem;
  categories: Category[];
  onDone: () => void;
}) {
  const commitments = useRecurringCommitments();
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const submit = async (write: RecurringCommitmentWrite) => {
    setError(null);
    try {
      await commitments.createCommitment(write);
      // The new commitment may have an occurrence near this line, so the
      // "Conciliar" row must see it.
      await queryClient.invalidateQueries({ queryKey: transactionReconciliationQueryKey(item.id) });
      onDone();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Não foi possível salvar o compromisso.");
    }
  };

  return (
    <>
      <PanelSection>
        <RecurringCommitmentFields
          formId={recurrenceFormId}
          commitment={null}
          initialDraft={recurringCommitmentPrefill(item)}
          categories={categories}
          submitError={error}
          onSubmit={(write) => void submit(write)}
        />
      </PanelSection>
      <PanelFooter>
        <Button type="primary" block htmlType="submit" form={recurrenceFormId} loading={commitments.isSaving}>
          Criar recorrência
        </Button>
      </PanelFooter>
    </>
  );
}

const manualFormId = "transaction-edit-manual-form";

/** Editing a lançamento manual in place of the overview, not in a second drawer. */
function EditManualScreen({
  item,
  accounts,
  categories,
  submitting,
  onSave,
  onDone,
}: {
  item: TransactionItem;
  accounts: Account[];
  categories: Category[];
  submitting: boolean;
  onSave: (id: string, write: ManualTransactionWrite) => Promise<void>;
  onDone: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const submit = async (write: ManualTransactionWrite) => {
    setError(null);
    try {
      await onSave(item.id, write);
      onDone();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Não foi possível salvar o lançamento.");
    }
  };
  return (
    <>
      <PanelSection>
        <ManualTransactionFields
          formId={manualFormId}
          transaction={item}
          accounts={accounts}
          categories={categories}
          submitError={error}
          onSubmit={(write) => void submit(write)}
        />
      </PanelSection>
      <PanelFooter>
        <Button type="primary" block htmlType="submit" form={manualFormId} loading={submitting}>
          Salvar
        </Button>
      </PanelFooter>
    </>
  );
}
