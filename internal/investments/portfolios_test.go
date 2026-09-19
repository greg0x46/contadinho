package investments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"contadinho-go/internal/investments"
)

func TestPortfolioProgressWithAndWithoutTarget(t *testing.T) {
	f := newBookFixture(t)
	ctx := context.Background()
	accountID := f.addManualAccount("Custódia")
	targetDate := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	withTarget, err := investments.CreatePortfolio(ctx, f.conn, investments.PortfolioInput{
		Name: "Reserva", TargetAmount: bookDecPtr("1000"), TargetDate: &targetDate,
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}
	withoutTarget, err := investments.CreatePortfolio(ctx, f.conn, investments.PortfolioInput{Name: "Aposentadoria"})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}
	if withTarget.CurrentValue.String() != "0" || bookDecimalString(withTarget.Progress) != "0" {
		t.Fatalf("empty goal = %+v", withTarget)
	}
	if withoutTarget.Progress != nil {
		t.Fatalf("goal without target reported progress: %v", withoutTarget.Progress)
	}

	if _, err := investments.CreatePosition(ctx, f.conn, investments.PositionInput{
		AccountID:       accountID,
		Name:            "Ações",
		AssetType:       "Ativo",
		PortfolioID:     &withTarget.ID,
		InitialQuantity: bookDec("10"),
		InitialUnitCost: bookDec("25"),
		OccurredOn:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreatePosition: %v", err)
	}
	reloaded, err := investments.GetPortfolio(ctx, f.conn, withTarget.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CurrentValue.String() != "250" || bookDecimalString(reloaded.Progress) != "0.25" {
		t.Fatalf("progress = %+v", reloaded)
	}
	if reloaded.TargetDate == nil || reloaded.TargetDate.Format("2006-01-02") != "2027-01-31" {
		t.Fatalf("target date = %v", reloaded.TargetDate)
	}
	list, err := investments.ListPortfolios(ctx, f.conn)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v err=%v", list, err)
	}
}

func TestUpdatePortfolioKeepsItsPositions(t *testing.T) {
	f := newBookFixture(t)
	ctx := context.Background()
	accountID := f.addManualAccount("Custódia")
	goal, err := investments.CreatePortfolio(ctx, f.conn, investments.PortfolioInput{Name: "Reserva", TargetAmount: bookDecPtr("1000")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := investments.CreatePosition(ctx, f.conn, investments.PositionInput{
		AccountID: accountID, Name: "CDB", AssetType: "Renda fixa", PortfolioID: &goal.ID, InitialValue: bookDecPtr("400"),
	}); err != nil {
		t.Fatalf("CreatePosition: %v", err)
	}
	updated, err := investments.UpdatePortfolio(ctx, f.conn, goal.ID, investments.PortfolioInput{Name: "Reserva de emergência"})
	if err != nil {
		t.Fatalf("UpdatePortfolio: %v", err)
	}
	if updated.Name != "Reserva de emergência" || updated.CurrentValue.String() != "400" || updated.Progress != nil {
		t.Fatalf("updated = %+v", updated)
	}
}

func TestDeletePortfolioKeepsPositionsAndBalances(t *testing.T) {
	f := newBookFixture(t)
	ctx := context.Background()
	accountID := f.addManualAccount("Custódia")
	f.addOperation(accountID, "", "initial_balance", "2026-09-01", "1000", "")
	goal, err := investments.CreatePortfolio(ctx, f.conn, investments.PortfolioInput{Name: "Reserva"})
	if err != nil {
		t.Fatal(err)
	}
	position, err := investments.CreatePosition(ctx, f.conn, investments.PositionInput{
		AccountID: accountID, Name: "CDB", AssetType: "Renda fixa", PortfolioID: &goal.ID, InitialValue: bookDecPtr("400"),
	})
	if err != nil {
		t.Fatalf("CreatePosition: %v", err)
	}
	before := f.summary()
	if err := investments.DeletePortfolio(ctx, f.conn, goal.ID); err != nil {
		t.Fatalf("DeletePortfolio: %v", err)
	}
	survivor, err := investments.GetPosition(ctx, f.conn, position.ID)
	if err != nil {
		t.Fatalf("position removed with the goal: %v", err)
	}
	if survivor.PortfolioID != nil || survivor.CurrentValue.String() != "400" {
		t.Fatalf("survivor = %+v", survivor)
	}
	after := f.summary()
	if after.TotalValue.String() != before.TotalValue.String() || after.CashBalance.String() != before.CashBalance.String() {
		t.Fatalf("deleting a goal changed balances: before=%+v after=%+v", before, after)
	}
	if _, err := investments.GetPortfolio(ctx, f.conn, goal.ID); !errors.Is(err, investments.ErrPortfolioNotFound) {
		t.Fatalf("get after delete = %v", err)
	}
}

func TestPortfolioRejectsEmptyName(t *testing.T) {
	f := newBookFixture(t)
	if _, err := investments.CreatePortfolio(context.Background(), f.conn, investments.PortfolioInput{Name: "  "}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Fatalf("create = %v", err)
	}
}
