import { Card } from "antd";
import { useState } from "react";

import type { InvestmentOperation, InvestmentOperationWrite } from "../../api/contracts";
import { useInvestmentWorkspace } from "../../hooks/useInvestmentWorkspace";
import { InvestmentOperationForm } from "./InvestmentOperationForm";
import { InvestmentOperationList } from "./InvestmentOperationList";

/**
 * The movimentações of the position backed by this synced investment. They
 * live here rather than in the account list so that page stays a summary;
 * correcting or deleting one happens where its position is shown in detail.
 */
export function InvestmentPositionOperations({ investmentId }: { investmentId: string }) {
  const workspace = useInvestmentWorkspace();
  const [editing, setEditing] = useState<InvestmentOperation | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const position = workspace.positions.find((candidate) => candidate.linked_investment_id === investmentId);
  if (workspace.isLoading || !position) return null;

  const operations = workspace.operations.filter((operation) => operation.position_id === position.id);
  if (operations.length === 0) return null;

  const busy = workspace.isSaving || workspace.isDeleting;

  const submit = async (write: InvestmentOperationWrite) => {
    if (!editing) return;
    setSaveError(null);
    try {
      await workspace.updateOperation({ operationId: editing.id, write });
      setEditing(null);
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Não foi possível salvar a movimentação.");
    }
  };
  const remove = async (operation: InvestmentOperation) => {
    setActionError(null);
    try {
      await workspace.deleteOperation(operation.id);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Não foi possível excluir a movimentação.");
    }
  };

  return (
    <Card title={`Movimentações registradas (${operations.length})`}>
      {actionError && <p role="alert">{actionError}</p>}
      <InvestmentOperationList
        operations={operations}
        positions={workspace.positions}
        onEdit={(operation) => {
          setSaveError(null);
          setEditing(operation);
        }}
        onDelete={(operation) => void remove(operation)}
        busy={busy}
      />
      <InvestmentOperationForm
        open={editing !== null}
        operation={editing}
        initial={editing === null ? null : { account_id: editing.account_id }}
        accounts={workspace.accounts}
        positions={workspace.positions}
        submitting={workspace.isSaving}
        submitError={saveError}
        onSubmit={(write) => void submit(write)}
        onSubmitCompound={() => undefined}
        onCancel={() => setEditing(null)}
      />
    </Card>
  );
}
