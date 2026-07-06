package reaction

import "testing"

func TestEntityRefValidate(t *testing.T) {
	if err := (EntityRef{Type: EntityArticle, ID: 1}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (EntityRef{Type: "bad", ID: 1}).Validate(); err != ErrInvalidEntityType {
		t.Fatalf("err = %v, want ErrInvalidEntityType", err)
	}
	if err := (EntityRef{Type: EntityArticle, ID: 0}).Validate(); err != ErrInvalidEntityID {
		t.Fatalf("err = %v, want ErrInvalidEntityID", err)
	}
}
