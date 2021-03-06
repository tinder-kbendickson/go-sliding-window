package swp

import (
	"time"
)

// RTT provides round-trip time estimation.
// Currently it is implemented as a simple single
// exponential moving average with alpha = 0.1
// and no seasonal/cyclic terms.
type RTT struct {
	Est   float64
	Alpha float64
	N     int64
	Sd    sdTracker
}

// NewRTT makes a new RTT.
func NewRTT() *RTT {
	return &RTT{
		Alpha: 0.1,
		Sd:    *newSdTracker(1),
	}
}

// GetEstimate returns the current estimate.
func (r *RTT) GetEstimate() time.Duration {
	return time.Duration(int64(r.Est))
}

// GetSd returns the standard deviation of
// the samples seen so far.
func (r *RTT) GetSd() time.Duration {
	switch r.N {
	case 0:
		return 10 * time.Millisecond
	case 1:
		return time.Duration(int64(r.Est / 2))
	}
	return time.Duration(int64(r.Sd.sd()[0]))

}

// AddSample adds a new RTT sample to the
// estimate.
func (r *RTT) AddSample(newSample time.Duration) {
	r.N++
	r.Sd.addObs([]float64{float64(newSample)}, 1.0)
	cur := float64(newSample)
	if r.N == 1 {
		r.Est = cur
		return
	}
	r.Est = r.Alpha*cur + (1.0-r.Alpha)*r.Est
}
