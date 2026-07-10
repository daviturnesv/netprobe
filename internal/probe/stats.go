package probe

type Stats struct {
	Sent     int
	Received int
	Lost     int
	LossPct  float64
}

type Percentiles struct {
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}