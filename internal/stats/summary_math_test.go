package stats

import "testing"

func TestDeriveFocusTag_Spike_HighestAndGreaterThan125Percent(t *testing.T) {
	tokens := int64(1000)
	cost := 10.0
	allTokens := []int64{1000, 600, 400, 200}
	allCosts := []float64{10.0, 8.0, 5.0, 2.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "spike" {
		t.Errorf("expected spike, got %s", tag)
	}
}

func TestDeriveFocusTag_Spike_HighestButNotEnough125Percent(t *testing.T) {
	tokens := int64(1000)
	cost := 10.0
	allTokens := []int64{1000, 850, 400, 200}
	allCosts := []float64{10.0, 8.0, 5.0, 2.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "--" {
		t.Errorf("expected --, got %s", tag)
	}
}

func TestDeriveFocusTag_Spike_NotHighest(t *testing.T) {
	tokens := int64(600)
	cost := 8.0
	allTokens := []int64{1000, 600, 400, 200}
	allCosts := []float64{10.0, 8.0, 5.0, 2.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag == "spike" {
		t.Errorf("expected not spike, got %s", tag)
	}
}

func TestDeriveFocusTag_Heavy_TokensAboveMedian(t *testing.T) {
	tokens := int64(2000)
	cost := 1.0
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{1.0, 1.0, 1.0, 1.0, 1.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "heavy" {
		t.Errorf("expected heavy, got %s", tag)
	}
}

func TestDeriveFocusTag_Heavy_CostAboveMedian(t *testing.T) {
	tokens := int64(500)
	cost := 20.0
	allTokens := []int64{500, 500, 500, 500, 500}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "heavy" {
		t.Errorf("expected heavy, got %s", tag)
	}
}

func TestDeriveFocusTag_Quiet_BothTokensAndCostBelowMedian(t *testing.T) {
	tokens := int64(100)
	cost := 0.5
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "quiet" {
		t.Errorf("expected quiet, got %s", tag)
	}
}

func TestDeriveFocusTag_Quiet_OnlyTokensBelow(t *testing.T) {
	tokens := int64(100)
	cost := 5.0
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag == "quiet" {
		t.Errorf("expected not quiet, got %s", tag)
	}
}

func TestDeriveFocusTag_NoTag_ZeroActivity(t *testing.T) {
	tokens := int64(0)
	cost := 0.0
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "--" {
		t.Errorf("expected --, got %s", tag)
	}
}

func TestDeriveFocusTag_SingleActiveDay(t *testing.T) {
	tokens := int64(1000)
	cost := 10.0
	allTokens := []int64{1000}
	allCosts := []float64{10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag == "spike" {
		t.Errorf("expected not spike for single day, got %s", tag)
	}
}

func TestDeriveFocusTag_AllZeroExceptOne(t *testing.T) {
	tokens := int64(500)
	cost := 5.0
	allTokens := []int64{500, 0, 0, 0, 0}
	allCosts := []float64{5.0, 0, 0, 0, 0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "--" {
		t.Errorf("expected --, got %s", tag)
	}
}

func TestCalculateMedian_OddLength(t *testing.T) {
	values := []int64{1, 2, 3, 4, 5}
	median := calculateMedian(values)
	if median != 3.0 {
		t.Errorf("expected 3.0, got %v", median)
	}
}

func TestCalculateMedian_EvenLength(t *testing.T) {
	values := []int64{1, 2, 3, 4}
	median := calculateMedian(values)
	if median != 2.5 {
		t.Errorf("expected 2.5, got %v", median)
	}
}

func TestCalculateMedian_Empty(t *testing.T) {
	values := []int64{}
	median := calculateMedian(values)
	if median != 0 {
		t.Errorf("expected 0, got %v", median)
	}
}

func TestCalculateMedianFloat_OddLength(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	median := calculateMedianFloat(values)
	if median != 3.0 {
		t.Errorf("expected 3.0, got %v", median)
	}
}

func TestCalculateMedianFloat_EvenLength(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0}
	median := calculateMedianFloat(values)
	if median != 2.5 {
		t.Errorf("expected 2.5, got %v", median)
	}
}
