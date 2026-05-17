package client

// ResultDetail contains the outcome of a Switcher evaluation, including the boolean result,
// a human-readable reason and optional metadata produced during evaluation.
type ResultDetail struct {
	Result   bool
	Reason   string
	Metadata map[string]any
}

// ToMap returns a serializable map representation of the ResultDetail.
func (r ResultDetail) ToMap() map[string]any {
	return map[string]any{
		"result":   r.Result,
		"reason":   r.Reason,
		"metadata": r.Metadata,
	}
}
