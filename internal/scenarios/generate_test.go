package scenarios_test

import (
	"errors"
	"testing"

	"contadinho-go/internal/scenarios"
)

func TestGenerateInstallmentsSumMatchesTotalWithLastAbsorbingRemainder(t *testing.T) {
	total := dec(t, "1000.00")
	got, err := scenarios.GenerateInstallments(total, 3, date(t, "2026-09-01"))
	if err != nil {
		t.Fatalf("GenerateInstallments: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}

	sum := dec(t, "0")
	for _, ins := range got {
		sum = sum.Add(ins.Amount)
	}
	if !sum.Equal(total) {
		t.Errorf("sum = %s, want %s", sum, total)
	}

	if !got[0].Amount.Equal(dec(t, "333.33")) || !got[1].Amount.Equal(dec(t, "333.33")) {
		t.Errorf("non-last installments = %s, %s, want 333.33 each", got[0].Amount, got[1].Amount)
	}
	if !got[2].Amount.Equal(dec(t, "333.34")) {
		t.Errorf("last installment = %s, want 333.34 (absorbs the remainder)", got[2].Amount)
	}
}

func TestGenerateInstallmentsEvenDivisionHasNoRemainder(t *testing.T) {
	got, err := scenarios.GenerateInstallments(dec(t, "900.00"), 3, date(t, "2026-09-01"))
	if err != nil {
		t.Fatalf("GenerateInstallments: %v", err)
	}
	for _, ins := range got {
		if !ins.Amount.Equal(dec(t, "300.00")) {
			t.Errorf("installment = %s, want 300.00", ins.Amount)
		}
	}
}

func TestGenerateInstallmentsMonthlyDates(t *testing.T) {
	got, err := scenarios.GenerateInstallments(dec(t, "300.00"), 3, date(t, "2026-01-31"))
	if err != nil {
		t.Fatalf("GenerateInstallments: %v", err)
	}
	wantDates := []string{"2026-01-31", "2026-03-03", "2026-03-31"} // AddDate normalizes Feb 31 -> Mar 3
	for i, ins := range got {
		if got := ins.ProjectedAt.Format("2006-01-02"); got != wantDates[i] {
			t.Errorf("installment[%d].ProjectedAt = %s, want %s", i, got, wantDates[i])
		}
	}
}

func TestGenerateInstallmentsRejectsInvalidInput(t *testing.T) {
	if _, err := scenarios.GenerateInstallments(dec(t, "0"), 3, date(t, "2026-01-01")); !errors.Is(err, scenarios.ErrInvalidInstallmentPlan) {
		t.Errorf("zero amount: err = %v, want ErrInvalidInstallmentPlan", err)
	}
	if _, err := scenarios.GenerateInstallments(dec(t, "-100"), 3, date(t, "2026-01-01")); !errors.Is(err, scenarios.ErrInvalidInstallmentPlan) {
		t.Errorf("negative amount: err = %v, want ErrInvalidInstallmentPlan", err)
	}
	if _, err := scenarios.GenerateInstallments(dec(t, "100"), 0, date(t, "2026-01-01")); !errors.Is(err, scenarios.ErrInvalidInstallmentPlan) {
		t.Errorf("zero months: err = %v, want ErrInvalidInstallmentPlan", err)
	}
}

func TestCreateGeneratedInstallmentsPersistsAll(t *testing.T) {
	conn := newTestDB(t)
	d := newDebt(t, conn)
	s, _ := scenarios.CreateScenario(t.Context(), conn, scenarios.KindDebtPlan, "Plano", &d.ID)

	drafts, err := scenarios.GenerateInstallments(dec(t, "1200.00"), 4, date(t, "2026-09-01"))
	if err != nil {
		t.Fatalf("GenerateInstallments: %v", err)
	}
	created, err := scenarios.CreateGeneratedInstallments(t.Context(), conn, s.ID, drafts)
	if err != nil {
		t.Fatalf("CreateGeneratedInstallments: %v", err)
	}
	if len(created) != 4 {
		t.Fatalf("len(created) = %d, want 4", len(created))
	}

	list, err := scenarios.ListScenarioTransactions(t.Context(), conn, s.ID)
	if err != nil {
		t.Fatalf("ListScenarioTransactions: %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("len(list) = %d, want 4", len(list))
	}
}
