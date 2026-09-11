package qac

// Resource describes a candidate cognitive resource.
type Resource struct {
	ID                         string
	Capability, Cost, Scarcity float64
	Available                  bool
	Metadata                   map[string]string
}
