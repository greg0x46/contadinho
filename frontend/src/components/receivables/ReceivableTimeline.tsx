import {
  Alert,
  Button,
  Card,
  DatePicker,
  Flex,
  InputNumber,
  Modal,
  Popconfirm,
  Radio,
  Select,
  Tag,
  Timeline,
  Typography,
} from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useState } from "react";

import type {
  Cadence,
  EligibleTransaction,
  Realization,
  ReceivableLink,
  ReceivableLinkedTransaction,
  ScenarioTransaction,
} from "../../api/contracts";
import { useReceivablePlan } from "../../hooks/useReceivablePlan";
import { formatOptionalDate } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";
import {
  scenarioTransactionStatusColor,
  scenarioTransactionStatusDashed,
  scenarioTransactionStatusLabel,
} from "../../presentation/scenarioLabels";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function formatProjectedDate(value: string): string {
  const [year, month, day] = value.split("-");
  return `${day}/${month}/${year}`;
}

function candidateLabel(candidate: EligibleTransaction): string {
  const date = formatOptionalDate(candidate.occurred_at);
  const description = candidate.description ?? "Sem descrição";
  const value = formatBRL(candidate.effective_money.value);
  return `${date} · ${description} · ${value}`;
}

function realizationLabel(realization: Realization, links: ReceivableLinkedTransaction[]): string {
  const link = links.find((candidate) => candidate.id === realization.receivable_link_id);
  const description = link?.description ?? "Transação vinculada";
  const date = link ? formatOptionalDate(link.occurred_at) : null;
  return date ? `${date} · ${description}` : description;
}

// Timeline's color prop only understands "blue"/"red"/"green"/"gray" (or a
// literal CSS color) — it doesn't share the Tag's semantic palette
// ("processing"/"success"/...), so installment status needs its own mapping
// just for the dot.
const timelineDotColor: Record<ScenarioTransaction["status"], string> = {
  atrasada: "red",
  projetada: "gray",
  paga_parcialmente: "orange",
  paga: "green",
  paga_a_mais: "blue",
};

function AccumulatedDeviationBanner({ value }: { value: string }) {
  const isAhead = value.startsWith("-");
  const isOnTrack = !isAhead && Number(value) === 0;
  const magnitude = formatBRL(isAhead ? value.slice(1) : value);

  if (isOnTrack) {
    return <Alert type="success" showIcon message="Em dia com o plano de recebimento até hoje." />;
  }
  return (
    <Alert
      type={isAhead ? "success" : "warning"}
      showIcon
      message={
        isAhead
          ? `Adiantado ${magnitude} em relação ao plano até hoje.`
          : `Atrasado ${magnitude} em relação ao plano até hoje.`
      }
    />
  );
}

// Lets a transaction that was linked standalone (no installment picked at
// link time) be allocated to a plan installment afterwards — the reverse
// direction of the combined vincular+alocar form below.
function AllocateExistingLinkControl({
  link,
  installments,
  onAllocate,
}: {
  link: ReceivableLinkedTransaction;
  installments: ScenarioTransaction[];
  onAllocate: (installmentId: string, amount: number) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [installmentId, setInstallmentId] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  if (!open) {
    return (
      <Button type="link" size="small" onClick={() => setOpen(true)}>
        Alocar a uma parcela
      </Button>
    );
  }

  const submit = async () => {
    if (installmentId === null) return;
    setSubmitting(true);
    try {
      await onAllocate(installmentId, Number(link.current_amount));
      setOpen(false);
      setInstallmentId(null);
    } catch {
      // The parent already surfaces the error via the shared action alert;
      // keep the form open so the user can adjust and retry.
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Flex gap="small" align="center" wrap>
      <Select
        aria-label="Parcela"
        style={{ minWidth: 220 }}
        placeholder="Escolha a parcela"
        value={installmentId}
        onChange={setInstallmentId}
        options={installments.map((installment) => ({
          value: installment.id,
          label: `${formatProjectedDate(installment.projected_at)} · ${installment.description}`,
        }))}
      />
      <Button type="primary" size="small" loading={submitting} disabled={installmentId === null} onClick={submit}>
        Alocar
      </Button>
      <Button size="small" onClick={() => setOpen(false)}>
        Cancelar
      </Button>
    </Flex>
  );
}

type TimelineEntry =
  | { kind: "installment"; date: string; installment: ScenarioTransaction }
  | { kind: "link"; date: string; link: ReceivableLinkedTransaction };

// ReceivableTimeline mirrors DebtTimeline: a receivable's installments and
// its linked (inflow) transactions are one chronological story, not two —
// an installment is "received" by the very links shown underneath it.
// Linking and allocating are also merged into a single "Vincular" action:
// picking a parcela at link time does both in one step, while leaving the
// parcela unset still links standalone.
export function ReceivableTimeline({
  receivableId,
  links,
  search,
  onSearchChange,
  candidates,
  isSearching,
  isLinking,
  onLinkTransaction,
  onUnlinkTransaction,
}: {
  receivableId: string;
  links: ReceivableLinkedTransaction[];
  search: string;
  onSearchChange: (value: string) => void;
  candidates: EligibleTransaction[];
  isSearching: boolean;
  isLinking: boolean;
  onLinkTransaction: (transactionId: string) => Promise<ReceivableLink>;
  onUnlinkTransaction: (linkId: string) => Promise<void>;
}) {
  const plan = useReceivablePlan(receivableId);
  const [cadence, setCadence] = useState<Cadence>("mensal");
  const [generateBy, setGenerateBy] = useState<"months" | "amount">("months");
  const [months, setMonths] = useState<number | null>(6);
  const [installmentAmount, setInstallmentAmount] = useState<number | null>(null);
  const [startDate, setStartDate] = useState<Dayjs>(() => dayjs());
  const [actionError, setActionError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deallocatingId, setDeallocatingId] = useState<string | null>(null);
  const [unlinkingLinkId, setUnlinkingLinkId] = useState<string | null>(null);

  const [addPaymentOpen, setAddPaymentOpen] = useState(false);
  const [selectedCandidateId, setSelectedCandidateId] = useState<string | null>(null);
  const [selectedInstallmentId, setSelectedInstallmentId] = useState<string>("none");
  const [linkSubmitting, setLinkSubmitting] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);

  const runAction = async (action: () => Promise<unknown>, fallback: string) => {
    setActionError(null);
    try {
      await action();
    } catch (error) {
      setActionError(errorMessage(error, fallback));
    }
  };

  const createPlan = () =>
    runAction(() => plan.createPlan("Plano de recebimento"), "Não foi possível criar o plano.");

  const generate = () => {
    const start_date = startDate.format("YYYY-MM-DD");
    if (generateBy === "months") {
      if (months === null || months < 1) return;
      return runAction(
        () => plan.generateInstallments({ cadence, months, start_date }),
        "Não foi possível gerar as parcelas.",
      );
    }
    if (installmentAmount === null || installmentAmount <= 0) return;
    return runAction(
      () => plan.generateInstallments({ cadence, installment_amount: installmentAmount, start_date }),
      "Não foi possível gerar as parcelas.",
    );
  };

  const readjust = (strategy: "abater_do_final" | "redistribuir") =>
    runAction(() => plan.readjust({ strategy }), "Não foi possível reajustar as parcelas restantes.");

  const deleteInstallment = async (installment: ScenarioTransaction) => {
    setDeletingId(installment.id);
    try {
      await plan.deleteInstallment(installment.id);
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível excluir a parcela."));
    } finally {
      setDeletingId(null);
    }
  };

  const deallocate = async (installment: ScenarioTransaction, realization: Realization) => {
    setDeallocatingId(realization.id);
    try {
      await plan.deallocateRealization({ transactionId: installment.id, realizationId: realization.id });
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível desalocar a transação."));
    } finally {
      setDeallocatingId(null);
    }
  };

  const unlink = async (link: ReceivableLinkedTransaction) => {
    setUnlinkingLinkId(link.id);
    try {
      await onUnlinkTransaction(link.id);
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível desvincular a transação."));
    } finally {
      setUnlinkingLinkId(null);
    }
  };

  const allocateExistingLink = async (linkId: string, installmentId: string, amount: number) => {
    try {
      await plan.allocateRealization({
        transactionId: installmentId,
        write: { receivable_link_id: linkId, allocated_amount: amount },
      });
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível alocar a transação."));
      throw error;
    }
  };

  const submitLink = async () => {
    if (selectedCandidateId === null) return;
    setLinkError(null);
    setLinkSubmitting(true);
    try {
      const link = await onLinkTransaction(selectedCandidateId);
      if (selectedInstallmentId !== "none") {
        await plan.allocateRealization({
          transactionId: selectedInstallmentId,
          write: { receivable_link_id: link.id, allocated_amount: Number(link.linked_amount) },
        });
      }
      setSelectedCandidateId(null);
      setSelectedInstallmentId("none");
      setAddPaymentOpen(false);
    } catch (error) {
      setLinkError(errorMessage(error, "Não foi possível vincular a transação."));
    } finally {
      setLinkSubmitting(false);
    }
  };

  const closeAddPayment = () => {
    setAddPaymentOpen(false);
    setSelectedCandidateId(null);
    setSelectedInstallmentId("none");
    setLinkError(null);
  };

  if (plan.isLoading) {
    return <Typography.Text type="secondary">Carregando linha do tempo…</Typography.Text>;
  }

  const installments = plan.plan?.transactions ?? [];
  const allocatedLinkIds = new Set(
    installments.flatMap((i) => i.realizations.map((r) => r.receivable_link_id)),
  );
  const unallocatedLinks = links.filter((link) => !allocatedLinkIds.has(link.id));

  const entries: TimelineEntry[] = [
    ...installments.map((installment) => ({
      kind: "installment" as const,
      date: installment.projected_at,
      installment,
    })),
    ...unallocatedLinks.map((link) => ({
      kind: "link" as const,
      date: link.occurred_at ?? link.linked_at,
      link,
    })),
  ].sort((a, b) => a.date.localeCompare(b.date));

  return (
    <Card
      title="Linha do tempo"
      extra={
        <Button type="primary" onClick={() => setAddPaymentOpen(true)}>
          Adicionar recebimento
        </Button>
      }
    >
    <Flex vertical gap="middle">
      {actionError && (
        <Alert type="error" showIcon closable onClose={() => setActionError(null)} message={actionError} />
      )}

      {plan.plan === null && (
        <Flex gap="small" align="center">
          <Typography.Text type="secondary">
            Esta conta a receber ainda não tem um plano de recebimento.
          </Typography.Text>
          <Button type="primary" loading={plan.isCreating} onClick={createPlan}>
            Criar plano de recebimento
          </Button>
        </Flex>
      )}

      {plan.plan !== null && plan.plan.transactions.length === 0 && (
        <Flex vertical gap="small">
          <Typography.Text>Gerar parcelas a partir do valor restante da conta a receber</Typography.Text>
          <Flex gap="small" align="center" wrap>
            <Typography.Text>Cadência:</Typography.Text>
            <Radio.Group
              value={cadence}
              onChange={(e) => setCadence(e.target.value as Cadence)}
              optionType="button"
            >
              <Radio.Button value="mensal">Mensal</Radio.Button>
              <Radio.Button value="semanal">Semanal</Radio.Button>
              <Radio.Button value="quinzenal">Quinzenal</Radio.Button>
            </Radio.Group>
          </Flex>
          <Radio.Group value={generateBy} onChange={(e) => setGenerateBy(e.target.value)}>
            <Radio value="months">Número de parcelas</Radio>
            <Radio value="amount">Valor da parcela</Radio>
          </Radio.Group>
          <Flex gap="small" align="center" wrap>
            <Typography.Text>Data de início:</Typography.Text>
            <DatePicker
              aria-label="Data de início"
              value={startDate}
              onChange={(value) => value && setStartDate(value)}
              format="DD/MM/YYYY"
              allowClear={false}
            />
          </Flex>
          <Flex gap="small" align="center" wrap>
            {generateBy === "months" ? (
              <>
                <InputNumber
                  aria-label="Número de parcelas"
                  min={1}
                  max={520}
                  value={months}
                  onChange={(value) => setMonths(value)}
                />
                <Typography.Text>parcelas</Typography.Text>
              </>
            ) : (
              <InputNumber
                aria-label="Valor de cada parcela"
                addonBefore="R$"
                min={0.01}
                step={0.01}
                value={installmentAmount}
                onChange={(value) => setInstallmentAmount(value)}
              />
            )}
            <Button type="primary" loading={plan.isGenerating} onClick={generate}>
              Gerar parcelas
            </Button>
          </Flex>
        </Flex>
      )}

      {plan.plan !== null && plan.plan.transactions.length > 0 && (
        <>
          <AccumulatedDeviationBanner value={plan.plan.accumulated_deviation} />
          <Flex gap="small" wrap>
            <Popconfirm
              title="Abater do final"
              description="Mantém o valor da parcela; o prazo (número de parcelas restantes) muda para cobrir o saldo."
              onConfirm={() => readjust("abater_do_final")}
              okText="Reajustar"
              cancelText="Cancelar"
            >
              <Button loading={plan.isReadjusting}>Reajustar: abater do final</Button>
            </Popconfirm>
            <Popconfirm
              title="Redistribuir entre os meses"
              description="Mantém o prazo (mesmas datas restantes); o valor de cada parcela muda para cobrir o saldo."
              onConfirm={() => readjust("redistribuir")}
              okText="Reajustar"
              cancelText="Cancelar"
            >
              <Button loading={plan.isReadjusting}>Reajustar: redistribuir entre os meses</Button>
            </Popconfirm>
          </Flex>
        </>
      )}

      <Modal
        title="Adicionar recebimento"
        open={addPaymentOpen}
        onCancel={closeAddPayment}
        onOk={submitLink}
        okText="Vincular"
        cancelText="Cancelar"
        okButtonProps={{ disabled: selectedCandidateId === null, loading: isLinking || linkSubmitting }}
        destroyOnHidden
      >
        <Flex vertical gap="middle">
          {linkError && <Alert type="error" showIcon message={linkError} />}
          <div>
            <Typography.Text>Transação</Typography.Text>
            <Select
              aria-label="Buscar transação para vincular"
              showSearch
              allowClear
              style={{ width: "100%" }}
              placeholder="Buscar por descrição"
              value={selectedCandidateId}
              searchValue={search}
              onSearch={onSearchChange}
              filterOption={false}
              loading={isSearching}
              notFoundContent={isSearching ? "Buscando…" : "Nenhuma transação elegível encontrada"}
              options={candidates.map((candidate) => ({ value: candidate.id, label: candidateLabel(candidate) }))}
              onChange={(value: string | null) => setSelectedCandidateId(value)}
            />
          </div>
          {installments.length > 0 && (
            <div>
              <Typography.Text>Parcela (opcional)</Typography.Text>
              <Select
                aria-label="Parcela para alocar"
                style={{ width: "100%" }}
                value={selectedInstallmentId}
                onChange={setSelectedInstallmentId}
                options={[
                  { value: "none", label: "Sem parcela (vínculo avulso)" },
                  ...installments.map((installment) => ({
                    value: installment.id,
                    label: `${formatProjectedDate(installment.projected_at)} · ${installment.description} · ${formatBRL(installment.amount)}`,
                  })),
                ]}
              />
            </div>
          )}
        </Flex>
      </Modal>

      {entries.length === 0 ? (
        <Typography.Text type="secondary">Nenhuma parcela ou transação vinculada ainda.</Typography.Text>
      ) : (
        <Timeline
          items={entries.map((entry) => {
            if (entry.kind === "installment") {
              const installment = entry.installment;
              return {
                color: timelineDotColor[installment.status],
                children: (
                  <Flex vertical gap={4}>
                    <Flex justify="space-between" align="center" wrap gap="small">
                      <span>
                        <strong>{formatProjectedDate(installment.projected_at)}</strong> ·{" "}
                        {installment.description} · {formatBRL(installment.amount)}
                      </span>
                      <Tag
                        color={scenarioTransactionStatusColor[installment.status]}
                        style={
                          scenarioTransactionStatusDashed[installment.status]
                            ? { borderStyle: "dashed" }
                            : undefined
                        }
                      >
                        {scenarioTransactionStatusLabel[installment.status]}
                      </Tag>
                    </Flex>
                    {installment.realizations.map((realization) => (
                      <Flex
                        key={realization.id}
                        justify="space-between"
                        align="center"
                        gap="small"
                        style={{ paddingLeft: 16 }}
                      >
                        <span>
                          ↳ {realizationLabel(realization, links)} · {formatBRL(realization.allocated_amount)}
                        </span>
                        <Popconfirm
                          title="Desalocar transação"
                          description="A parcela deixa de contar esse valor como recebido; a transação em si permanece vinculada à conta a receber."
                          onConfirm={() => deallocate(installment, realization)}
                          okText="Desalocar"
                          cancelText="Cancelar"
                        >
                          <Button type="link" danger size="small" loading={deallocatingId === realization.id}>
                            Desalocar
                          </Button>
                        </Popconfirm>
                      </Flex>
                    ))}
                    <Popconfirm
                      title="Excluir parcela"
                      description="A parcela planejada será removida do plano."
                      onConfirm={() => deleteInstallment(installment)}
                      okText="Excluir"
                      cancelText="Cancelar"
                    >
                      <Button type="link" danger size="small" loading={deletingId === installment.id}>
                        Excluir parcela
                      </Button>
                    </Popconfirm>
                  </Flex>
                ),
              };
            }

            const link = entry.link;
            return {
              color: "gray",
              children: (
                <Flex vertical gap={4}>
                  <Flex justify="space-between" align="center" wrap gap="small">
                    <span>
                      {formatOptionalDate(link.occurred_at)} · {link.description ?? "Sem descrição"} ·{" "}
                      {formatBRL(link.current_amount)}
                    </span>
                    <Tag>Vínculo avulso</Tag>
                  </Flex>
                  <Flex gap="small" align="center" wrap>
                    <Popconfirm
                      title="Desvincular transação"
                      description="A transação permanece inalterada e volta a ficar disponível para vincular."
                      onConfirm={() => unlink(link)}
                      okText="Desvincular"
                      cancelText="Cancelar"
                    >
                      <Button type="link" danger size="small" loading={unlinkingLinkId === link.id}>
                        Desvincular
                      </Button>
                    </Popconfirm>
                    {installments.length > 0 && (
                      <AllocateExistingLinkControl
                        link={link}
                        installments={installments}
                        onAllocate={(installmentId, amount) => allocateExistingLink(link.id, installmentId, amount)}
                      />
                    )}
                  </Flex>
                </Flex>
              ),
            };
          })}
        />
      )}
    </Flex>
    </Card>
  );
}
