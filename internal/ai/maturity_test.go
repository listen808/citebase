package ai

import "testing"

func TestMaturityLevel_Gte(t *testing.T) {
	cases := []struct {
		name     string
		a        MaturityLevel
		b        MaturityLevel
		expected bool
	}{
		{"stable >= experimental", MaturityStable, MaturityExperimental, true},
		{"stable >= beta", MaturityStable, MaturityBeta, true},
		{"stable >= stable", MaturityStable, MaturityStable, true},
		{"beta >= experimental", MaturityBeta, MaturityExperimental, true},
		{"beta >= beta", MaturityBeta, MaturityBeta, true},
		{"beta >= stable", MaturityBeta, MaturityStable, false},
		{"experimental >= experimental", MaturityExperimental, MaturityExperimental, true},
		{"experimental >= beta", MaturityExperimental, MaturityBeta, false},
		{"experimental >= stable", MaturityExperimental, MaturityStable, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.a.Gte(tc.b)
			if result != tc.expected {
				t.Errorf("%s.Gte(%s) = %v, expected %v", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

func TestMaturityLevel_String(t *testing.T) {
	if MaturityStable.String() != "stable" {
		t.Errorf("expected 'stable', got '%s'", MaturityStable.String())
	}
	if MaturityBeta.String() != "beta" {
		t.Errorf("expected 'beta', got '%s'", MaturityBeta.String())
	}
	if MaturityExperimental.String() != "experimental" {
		t.Errorf("expected 'experimental', got '%s'", MaturityExperimental.String())
	}
}

func TestFilterFeaturesByMaturity(t *testing.T) {
	features := []FeatureMaturity{
		{Name: "f1", Maturity: MaturityStable},
		{Name: "f2", Maturity: MaturityBeta},
		{Name: "f3", Maturity: MaturityExperimental},
		{Name: "f4", Maturity: MaturityStable},
	}

	t.Run("filter stable only", func(t *testing.T) {
		result := FilterFeaturesByMaturity(features, MaturityStable)
		if len(result) != 2 {
			t.Errorf("expected 2 stable features, got %d", len(result))
		}
		for _, f := range result {
			if f.Maturity != MaturityStable {
				t.Errorf("expected stable only, got %s", f.Maturity)
			}
		}
	})

	t.Run("filter beta and above", func(t *testing.T) {
		result := FilterFeaturesByMaturity(features, MaturityBeta)
		if len(result) != 3 {
			t.Errorf("expected 3 beta+ features, got %d", len(result))
		}
	})

	t.Run("filter experimental (all)", func(t *testing.T) {
		result := FilterFeaturesByMaturity(features, MaturityExperimental)
		if len(result) != 4 {
			t.Errorf("expected all 4 features, got %d", len(result))
		}
	})
}

func TestGetStableRAGFeatures(t *testing.T) {
	result := GetStableRAGFeatures()
	for _, f := range result {
		if f.Maturity != MaturityStable {
			t.Errorf("GetStableRAGFeatures returned non-stable feature: %s (%s)", f.Name, f.Maturity)
		}
	}
}

func TestGetStableTools(t *testing.T) {
	result := GetStableTools()
	for name, level := range result {
		if level != MaturityStable {
			t.Errorf("GetStableTools returned non-stable tool: %s (%s)", name, level)
		}
	}
	if len(result) == 0 {
		t.Error("GetStableTools returned empty map")
	}
}

func TestRAGFeatures_HaveValidMaturity(t *testing.T) {
	validLevels := map[MaturityLevel]bool{
		MaturityStable:       true,
		MaturityBeta:         true,
		MaturityExperimental: true,
	}
	for _, f := range RAGFeatures {
		if !validLevels[f.Maturity] {
			t.Errorf("RAG feature %s has invalid maturity: %s", f.Name, f.Maturity)
		}
		if f.Name == "" {
			t.Error("found RAG feature with empty name")
		}
		if f.Category == "" {
			t.Errorf("RAG feature %s has empty category", f.Name)
		}
	}
}

func TestToolMaturity_HaveValidLevels(t *testing.T) {
	validLevels := map[MaturityLevel]bool{
		MaturityStable:       true,
		MaturityBeta:         true,
		MaturityExperimental: true,
	}
	for name, level := range ToolMaturity {
		if !validLevels[level] {
			t.Errorf("tool %s has invalid maturity: %s", name, level)
		}
	}
}

func TestText2SQLFeatures_HaveValidMaturity(t *testing.T) {
	validLevels := map[MaturityLevel]bool{
		MaturityStable:       true,
		MaturityBeta:         true,
		MaturityExperimental: true,
	}
	for _, f := range Text2SQLFeatures {
		if !validLevels[f.Maturity] {
			t.Errorf("Text2SQL feature %s has invalid maturity: %s", f.Name, f.Maturity)
		}
		if f.Name == "" {
			t.Error("found Text2SQL feature with empty name")
		}
	}
}

func TestAnalyzerFeatures_HaveValidMaturity(t *testing.T) {
	validLevels := map[MaturityLevel]bool{
		MaturityStable:       true,
		MaturityBeta:         true,
		MaturityExperimental: true,
	}
	for _, f := range AnalyzerFeatures {
		if !validLevels[f.Maturity] {
			t.Errorf("Analyzer feature %s has invalid maturity: %s", f.Name, f.Maturity)
		}
		if f.Name == "" {
			t.Error("found Analyzer feature with empty name")
		}
	}
}
