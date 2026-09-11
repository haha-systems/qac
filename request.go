package qac

// Context contains the signals used to allocate a request.
type Context struct {
	CurrentResource                                string
	Uncertainty, Importance, Novelty, ExpectedGain float64
	FailedAttempts                                 int
	Metadata                                       map[string]string
}

// ResourceBudget states whether a resource can receive more work.
type ResourceBudget struct {
	Enabled              bool
	RemainingInvocations *int
	Cooldown             bool
}

// BudgetState contains resource budgets by resource ID.
type BudgetState struct {
	Resources map[string]ResourceBudget
}

// Request contains a context, resource candidates, and their budgets.
type Request struct {
	Context   Context
	Resources []Resource
	Budget    BudgetState
}
