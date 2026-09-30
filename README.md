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

## Jev client

The optional `jev` package sends typed Choice, Score, and Noul questions to the
TypeSafe System One API. It returns the model's probabilities and confidence;
QAC callers remain responsible for how to use those values.

Set `TYPESAFE_API_KEY` and run the client example:

```sh
go run ./examples/jev-client
```

The client uses `jev-latest` and `https://api.typesafe.ai` by default. The API
schema allows one or more Score levels. The guide recommends two or more, caps
Score at 10 levels, and caps Choice at 255 options; the client follows the
published schema when it differs from that guidance.

For authenticated integration checks, also set `JEV_LIVE_TEST=1` and run
`go test ./jev -run TestLiveAPI -count=1`. Normal tests use local HTTP servers.

## Jev policy

`policy/jev` implements `qac.Policy`. It uses the hierarchy and rules in a
`threshold.Config` to enforce eligibility and to make a deterministic decision
when Jev's call fails, times out, returns an invalid answer, or has confidence
below the configured floor. The default floor is `.5`; the default call limit
is five seconds. Jev may choose any eligible resource in the hierarchy, and
QAC maps its position to `continue`, `escalate`, or `release`.

The policy sends numeric task signals and only the `description` entry from
context and resource metadata. It sends eligible resource IDs, their
descriptions, capability, cost, and scarcity. It sends no other metadata or
budget values. If more than 255 resources are eligible, it uses the threshold
policy without calling Jev. A returned decision includes Jev's model,
confidence, and full probability distribution. An empty model identifies a
deterministic threshold decision.

The `examples/jev-client` program shows the policy integration. Local policy
tests use a fake Jev client; they do not call the network. To run the live
incident-routing scenario with Luna, Sol, and Astra, set `TYPESAFE_API_KEY`
and `JEV_LIVE_TEST=1`, then run `go test ./policy/jev -run TestLiveResourceSelection -count=1 -v`.

## Non-goals

The core QAC policies do not mutate budgets. The threshold policy does not make
network calls; the optional Jev client and policy do.

## License

QAC is licensed under the Apache License 2.0. See [LICENSE](LICENSE).
