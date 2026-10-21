package domain

// Millicores is a CPU quantity in thousandths of a core.
type Millicores int64

// Bytes is a memory or storage quantity.
type Bytes int64

// PIDs is a process-count limit.
type PIDs int64

// GPURequest requests identical devices from one GPU class.
type GPURequest struct {
	Vendor string
	Class  string
	Count  int
}

// Resources is the normalized resource request shared by compute backends.
type Resources struct {
	CPUMillicores Millicores
	Memory        Bytes
	PIDs          PIDs
	GPUs          []GPURequest
}

// Validate checks that non-zero quantities are positive.
func (r Resources) Validate(op string) error {
	if r.CPUMillicores < 0 {
		return Invalidf(op, "cpu must not be negative")
	}
	if r.Memory < 0 {
		return Invalidf(op, "memory must not be negative")
	}
	if r.PIDs < 0 {
		return Invalidf(op, "pids must not be negative")
	}
	for _, gpu := range r.GPUs {
		if gpu.Count < 1 {
			return Invalidf(op, "gpu count must be positive")
		}
	}
	return nil
}
