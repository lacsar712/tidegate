package level

// HeadDiff returns absolute upstream-downstream difference in centimeters.
func HeadDiff(upstreamCM, downstreamCM int) int {
	diff := upstreamCM - downstreamCM
	if diff < 0 {
		return -diff
	}
	return diff
}

// WithinLimit reports whether head difference is at or below the configured limit.
func WithinLimit(headDiffCM, limitCM int) bool {
	return headDiffCM <= limitCM
}

// ExceedsLimit is the inverse helper for denial messaging.
func ExceedsLimit(headDiffCM, limitCM int) bool {
	return headDiffCM >= limitCM
}
