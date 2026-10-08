import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import * as accountsApi from "../../api/accounts";
import { QueryTestProvider } from "../../test/QueryTestProvider";
import { RecurringCommitmentFields } from "./RecurringCommitmentForm";

vi.mock("../../api/accounts");

function renderFields(submitting: boolean) {
  vi.mocked(accountsApi.listAccounts).mockResolvedValue([]);
  const { container } = render(
    <QueryTestProvider>
      <RecurringCommitmentFields
        formId="recurrence-form"
        commitment={null}
        categories={[]}
        submitError={null}
        submitting={submitting}
        onSubmit={vi.fn()}
      />
    </QueryTestProvider>,
  );
  return container.querySelector("form") as HTMLFormElement;
}

describe("RecurringCommitmentFields", () => {
  it("validates on submit (the empty draft is refused with a message)", () => {
    const form = renderFields(false);
    fireEvent.submit(form);
    expect(form.querySelector(".ant-alert")).not.toBeNull();
  });

  it("ignores a submit — Enter in a field — while a write is already in flight", () => {
    const form = renderFields(true);
    fireEvent.submit(form);
    expect(form.querySelector(".ant-alert")).toBeNull();
  });
});
