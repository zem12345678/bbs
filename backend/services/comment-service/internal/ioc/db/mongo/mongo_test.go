package mongo

import "testing"

func TestHasConnectionTargetAcceptsURIWithoutEndpoints(t *testing.T) {
	if !hasConnectionTarget(&Options{URI: " mongodb://mongo:27017/?authSource=admin "}) {
		t.Fatal("hasConnectionTarget() = false for URI-only config")
	}
}

func TestHasConnectionTargetRejectsBlankConfig(t *testing.T) {
	if hasConnectionTarget(&Options{Endpoints: []string{"", " "}}) {
		t.Fatal("hasConnectionTarget() = true for blank endpoints")
	}
}

func TestCleanEndpointsTrimsBlankValues(t *testing.T) {
	got := cleanEndpoints([]string{" mongo-a:27017 ", "", "mongo-b:27017"})
	if len(got) != 2 || got[0] != "mongo-a:27017" || got[1] != "mongo-b:27017" {
		t.Fatalf("cleanEndpoints() = %#v", got)
	}
}
