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
	// ActionSetCategory assigns a category to a matched transaction — see
	// internal/categories.ApplyRule.
	ActionSetCategory ActionType = "set_category"
)

// Action is one automation_rule_actions row. A rule may carry more than one
// Action (e.g. set_category + reconcile together).
type Action struct {
	ID                    string
	Type                  ActionType
	RecurringCommitmentID *string // set iff Type == ActionReconcile
	CategoryID            *string // set iff Type == ActionSetCategory
}

// ActionWrite is the create/update-time shape of an Action — no ID or
// position, the store assigns position by slice order (mirroring how
// conditions are written).
type ActionWrite struct {
	Type                  ActionType
	RecurringCommitmentID *string
	CategoryID            *string
}
