package client

type ResultDetail struct {
	Result   bool
	Reason   string
	Metadata map[string]any
}

func (r ResultDetail) ToMap() map[string]any {
	return map[string]any{
		"result":   r.Result,
		"reason":   r.Reason,
		"metadata": r.Metadata,
	}
}
