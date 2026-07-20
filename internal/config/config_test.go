package config

import "testing"

func TestLoadUsesSemanticSimilarityThresholdByDefault(t *testing.T) {
	t.Setenv("DOCMIND_RETRIEVAL_THRESHOLD", "")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.RetrievalThreshold != 0.10 {
		t.Fatalf("threshold = %v, want 0.10", config.RetrievalThreshold)
	}
}

func TestLoadRejectsOutOfRangeSimilarityThreshold(t *testing.T) {
	t.Setenv("DOCMIND_RETRIEVAL_THRESHOLD", "1.01")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a threshold above one")
	}
}
