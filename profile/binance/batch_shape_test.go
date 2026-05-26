package binance

import (
	"reflect"
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
)

func TestNewBatchShapeProductionDefault(t *testing.T) {
	t.Setenv(shapeOverrideEnv, "")
	got := NewBatchShape().Shapes()
	want := []spec.BatchShape{
		{AssetCountTier: 50, UsersPerBatch: 700},
		{AssetCountTier: 500, UsersPerBatch: 92},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default shapes: got %#v, want %#v", got, want)
	}
}

func TestNewBatchShapeOverrideSingle(t *testing.T) {
	t.Setenv(shapeOverrideEnv, "5_10")
	got := NewBatchShape().Shapes()
	want := []spec.BatchShape{{AssetCountTier: 5, UsersPerBatch: 10}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("override shapes: got %#v, want %#v", got, want)
	}
}

func TestNewBatchShapeOverrideMultiSortsAscending(t *testing.T) {
	t.Setenv(shapeOverrideEnv, "500_92,5_10")
	got := NewBatchShape().Shapes()
	want := []spec.BatchShape{
		{AssetCountTier: 5, UsersPerBatch: 10},
		{AssetCountTier: 500, UsersPerBatch: 92},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("override shapes: got %#v, want %#v", got, want)
	}
}

func TestParseShapeOverrideErrors(t *testing.T) {
	cases := map[string]string{
		"empty entry":    ",5_10",
		"missing under":  "510",
		"too many parts": "5_10_20",
		"tier zero":      "0_10",
		"tier negative":  "-1_10",
		"users zero":     "5_0",
		"tier nonint":    "abc_10",
		"users nonint":   "5_xyz",
		"duplicate tier": "5_10,5_20",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseShapeOverride(raw)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", raw)
			}
		})
	}
}
