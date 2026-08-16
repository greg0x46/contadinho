package automation

// ActionType is what a Rule does when its conditions match.
type ActionType string

const (
	// ActionIgnore marks a matched transaction as ignored — the sole
	// behavior automation rules had before actions existed.
	ActionIgnore ActionType = "ignore"
	// ActionReconcile links the rule to a recurring commitment: the rule's
	// conditions are used by internal/recurrences to resolve that
	// commitment's occurrences against real transactions at read time.
	ActionReconcile ActionType = "reconcile"
)

// Action is one automation_rule_actions row.
type Action struct {
	ID                    string
	Type                  ActionType
	RecurringCommitmentID *string // set iff Type == ActionReconcile
}

// ActionWrite is the create/update-time shape of an Action — no ID or
// position, the store assigns position by slice order (mirroring how
// conditions are written).
type ActionWrite struct {
	Type                  ActionType
	RecurringCommitmentID *string
}
