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
	ID   string
	Type ActionType
	// ScenarioID is the canonical target for reconcile actions. It must
	// reference a Scenario of kind recurring.
	ScenarioID *string
	// RecurringCommitmentID is retained while the legacy API is alive. New
	// rows may leave it nil; reads expose it when an old commitment mapping
	// exists so old clients can keep working.
	RecurringCommitmentID *string
	CategoryID            *string // set iff Type == ActionSetCategory
}

// ActionWrite is the create/update-time shape of an Action — no ID or
// position, the store assigns position by slice order (mirroring how
// conditions are written).
type ActionWrite struct {
	Type                  ActionType
	ScenarioID            *string
	RecurringCommitmentID *string
	CategoryID            *string
}
