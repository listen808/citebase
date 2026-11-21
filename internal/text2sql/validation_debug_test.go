package text2sql

import (
	"fmt"
	"testing"
)

func TestValidateSQLStructureDebug(t *testing.T) {
	goodSQL := "SELECT p.param_val, p.param_desc, d.param_value FROM bm_dc_param p INNER JOIN bm_data_dict d ON p.param_val = d.param_value WHERE p.param_val LIKE '%caftp%'"
	issues := ValidateSQLStructure(goodSQL, []string{"bm_dc_param", "bm_data_dict"})
	fmt.Printf("Issues: %v\\n", issues)
	for _, iss := range issues {
		if iss.Type == "error" {
			t.Errorf("unexpected error: %s", iss.Message)
		}
	}
}
