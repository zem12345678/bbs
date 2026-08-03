package reaction

import (
	"strings"
	"testing"
)

func TestValidCollectionEntityType(t *testing.T) {
	for _, entityType := range []EntityType{EntityArticle, EntityTopic} {
		if !ValidCollectionEntityType(entityType) {
			t.Fatalf("ValidCollectionEntityType(%q) = false", entityType)
		}
	}
	for _, entityType := range []EntityType{EntityComment, "unknown", ""} {
		if ValidCollectionEntityType(entityType) {
			t.Fatalf("ValidCollectionEntityType(%q) = true", entityType)
		}
	}
}

func TestValidateCollectionFieldsNormalizesAndValidates(t *testing.T) {
	name, description, err := ValidateCollectionFields(42, "  Reading  ", "  Notes  ")
	if err != nil {
		t.Fatalf("ValidateCollectionFields() error = %v", err)
	}
	if name != "Reading" || description != "Notes" {
		t.Fatalf("normalized fields = %q, %q", name, description)
	}

	tests := []struct {
		name        string
		userID      int64
		collection  string
		description string
		want        error
	}{
		{name: "invalid user", collection: "Reading", want: ErrInvalidUserID},
		{name: "blank name", userID: 1, collection: "   ", want: ErrInvalidCollectionName},
		{name: "long name", userID: 1, collection: strings.Repeat("名", MaxCollectionNameRunes+1), want: ErrInvalidCollectionName},
		{name: "long description", userID: 1, collection: "Reading", description: strings.Repeat("注", MaxCollectionDescriptionRunes+1), want: ErrInvalidCollectionDescription},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ValidateCollectionFields(tt.userID, tt.collection, tt.description)
			if err != tt.want {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestEntityRefValidateForCollection(t *testing.T) {
	if err := (EntityRef{Type: EntityArticle, ID: 1}).ValidateForCollection(); err != nil {
		t.Fatalf("article validation error = %v", err)
	}
	if err := (EntityRef{Type: EntityComment, ID: 1}).ValidateForCollection(); err != ErrInvalidCollectionEntityType {
		t.Fatalf("comment validation error = %v", err)
	}
	if err := (EntityRef{Type: EntityTopic, ID: 0}).ValidateForCollection(); err != ErrInvalidEntityID {
		t.Fatalf("zero ID validation error = %v", err)
	}
}
