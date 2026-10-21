package openshell

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestValidateRegexMiddlewareConfig(t *testing.T) {
	for name, config := range map[string]*structpb.Struct{
		"default":         nil,
		"explicit redact": {Fields: map[string]*structpb.Value{"mode": structpb.NewStringValue("redact")}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateRegexMiddlewareConfig(config); err != nil {
				t.Fatal(err)
			}
		})
	}
	invalid, err := structpb.NewStruct(map[string]any{"mode": "replace"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRegexMiddlewareConfig(invalid); err == nil {
		t.Fatal("unsupported mode was accepted")
	}
	unknown, err := structpb.NewStruct(map[string]any{"mode": "redact", "pattern": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRegexMiddlewareConfig(unknown); err == nil {
		t.Fatal("unknown config field was accepted")
	}
}
