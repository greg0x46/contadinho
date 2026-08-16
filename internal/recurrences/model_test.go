package recurrences_test

import "testing"

func TestValidateStillRequiresDayOfMonthField(t *testing.T) {
	c := baseCommitment(t)
	c.DayOfMonth = 0
	if err := c.Validate(); err == nil {
		t.Error("expected an error when the descriptive day_of_month is unset — it's the scheduling anchor, always required")
	}
}
