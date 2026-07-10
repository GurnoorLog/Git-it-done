package fireworks

import (
	"testing"
)

func TestClassifyTiers(t *testing.T) {
	// The 5 real models from the PRD update.
	// Used here STRICTLY as test fixtures to verify the generic pattern matcher,
	// maintaining compliance with the "no hardcoded model IDs in source" rule.
	allowed := []string{
		"minimax-m3",
		"kimi-k2p7-code",
		"gemma-4-31b-it",
		"gemma-4-26b-a4b-it",
		"gemma-4-31b-it-nvfp4",
	}

	tm := ClassifyTiers(allowed)

	tests := []struct {
		tier Tier
		want string
	}{
		{TierCheap, "gemma-4-26b-a4b-it"},       // matches MoE aXb pattern
		{TierQuantized, "gemma-4-31b-it-nvfp4"}, // matches nvfp4 pattern
		{TierCode, "kimi-k2p7-code"},            // matches code pattern
		{TierDense, "gemma-4-31b-it"},           // residual Gemma
		{TierFlagship, "minimax-m3"},            // absolute residual
	}

	for _, tc := range tests {
		got, ok := tm.Models[tc.tier]
		if !ok {
			t.Errorf("Tier %s missing from map", tc.tier)
		} else if got != tc.want {
			t.Errorf("Tier %s = %q, want %q", tc.tier, got, tc.want)
		}
	}
}

func TestSelectModel(t *testing.T) {
	allowed := []string{
		"minimax-m3",
		"kimi-k2p7-code",
		"gemma-4-31b-it",
		"gemma-4-26b-a4b-it",
		"gemma-4-31b-it-nvfp4",
	}
	tm := ClassifyTiers(allowed)

	tests := []struct {
		category string
		want     string
	}{
		{"sentiment", "minimax-m3"},           // Prefers Flagship
		{"ner", "minimax-m3"},                 // Prefers Flagship
		{"summarization", "minimax-m3"},       // Prefers Flagship
		{"factual", "minimax-m3"},             // Prefers Flagship
		{"code_generation", "kimi-k2p7-code"}, // Prefers Code
		{"code_debugging", "kimi-k2p7-code"},  // Prefers Code
		{"math", "minimax-m3"},                // Prefers Flagship
		{"logical", "minimax-m3"},             // Prefers Flagship
	}

	for _, tc := range tests {
		got := tm.SelectModel(tc.category)
		if got != tc.want {
			t.Errorf("SelectModel(%q) = %q, want %q", tc.category, got, tc.want)
		}
	}
}
