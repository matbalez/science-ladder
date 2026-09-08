package platform

import "testing"

func TestOperatorPreparationAllowanceCannotExpandUserBudget(t *testing.T) {
	for _, tc := range []struct {
		role             string
		configured, want int
	}{
		{"solver", 100, 20}, {"creator", 100, 20}, {"editor", 100, 20}, {"operator", 0, 20}, {"operator", 100, 100}, {"operator", 10000, 1000},
	} {
		if got := preparationDailyLimit(tc.role, tc.configured); got != tc.want {
			t.Fatalf("%s/%d: got %d, want %d", tc.role, tc.configured, got, tc.want)
		}
	}
}
