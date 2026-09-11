# QAC

QAC is a dependency-free Go library for deterministic allocation decisions
between constrained cognitive resources. It evaluates supplied signals and
resource limits. It does not invoke resources, perform I/O, or mutate budgets.

## Domain terms

A `Resource` is a candidate allocation target. Its `Capability`, `Cost`, and
`Scarcity` are normalized values in `[0,1]`. A `Context` identifies the current
resource and provides normalized uncertainty, importance, novelty, and
expected-gain signals. A `Request` combines these with a `BudgetState`.

A `Policy` returns a `Decision`: `continue`, `escalate`, `release`, or `stop`.
Each decision names its source and destination, score, threshold, factors,
reason, and eligibility evidence.

## Quick start

```go
policy, err := threshold.New(threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
if err != nil {
	return err
}
decision, err := policy.Decide(context.Background(), request)
if err != nil {
	return err
}
fmt.Println(decision.Action, decision.From, decision.To)
```

Run the complete three-tier example:

```sh
go run ./examples/tiers
```

```text
escalate wraith -> shade
escalate shade -> veil
release veil -> shade
```

## Threshold scoring

`policy/threshold` is immutable and safe for concurrent use. It evaluates
adjacent hierarchy moves only, checks an upward move first, and uses separate
escalation and release thresholds to provide hysteresis.

For the candidate destination, the score is:

```text
pressure - (costWeight * cost + scarcityWeight * scarcity) * (1 - expectedGain)
```

Pressure is the weighted sum of uncertainty, importance, novelty, expected
gain, and saturated failed attempts. Default weights are `.25`, `.15`, `.10`,
`.35`, and `.15`; cost and scarcity weights are `.20` and `.30`. Failures
saturate at three attempts. Default escalation thresholds are `.55` then `.80`;
release thresholds are `.25` then `.45`.

## Budget constraints

The policy evaluates eligibility before transitions. A resource is ineligible
when it is unavailable, outside the hierarchy, disabled, cooling down, or has
no remaining invocations. It does not change the supplied budget. If no
resource is eligible, the policy returns `stop`.

For an ineligible current resource, configure `OnCurrentIneligible` with
`stop`, `nearest_eligible`, or `least_cost_eligible`. The default is `stop`.

## Decision evidence

`Decision.Eligibility` reports every resource and its stable ineligibility
reasons. `Decision.Factors` lists weighted contributions that sum to
`Decision.Score`. `Decision.Reason` records the action, transition, score,
threshold, and three largest factor contributions.

## Custom policies

QAC depends only on the public `Policy` contract:

```go
type Policy interface {
	Decide(context.Context, Request) (Decision, error)
}
```

Use `threshold.New` for the built-in policy, or provide another implementation
that returns the same inspectable decision data.

## Non-goals

QAC does not call models or tools, read configuration, perform network or file
operations, log, mutate budgets, or encode provider-specific workflows.

## License

QAC is licensed under the Apache License 2.0. See [LICENSE](LICENSE).
