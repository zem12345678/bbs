package persistence

import (
	"strings"
	"testing"
)

func TestCartActiveProductConditionFiltersProductStatus(t *testing.T) {
	condition := cartActiveProductCondition("p")
	if !strings.Contains(condition, "p.status = $2") {
		t.Fatalf("cart active product condition = %q, want product status filter", condition)
	}
}
