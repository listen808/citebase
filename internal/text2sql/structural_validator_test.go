package text2sql

import (
	"testing"
)

func TestValidateSQLStructure_DetectsDuplicateAndAliasIssues(t *testing.T) {
	badSQL := "SELECT PARAM, param_val, codea, param_desc, codeb, param_id, param_code, stype, cfg_ver_id, status_cd, PARAM_VALUE, ORDER_ID, ORDER_NBR, ORDER_ID, ORDER_NBR, PARAM_VAL, PARAM_DESC FROM bm_dc_param"

	issues := ValidateSQLStructure(badSQL, []string{"bm_dc_param", "BM_DATA_DICT"})

	hasDuplicate := false
	for _, iss := range issues {
		if iss.Type == "error" && contains(iss.Message, "重复列") {
			hasDuplicate = true
		}
	}

	if !hasDuplicate {
		t.Errorf("应该检测到重复列 ORDER_ID/ORDER_NBR，但没找到。issues: %v", issues)
	}
}

func TestValidateSQLStructure_ValidSQL(t *testing.T) {
	goodSQL := "SELECT p.param_val, p.param_desc, d.param_value FROM bm_dc_param p INNER JOIN bm_data_dict d ON p.param_val = d.param_value WHERE p.param_val LIKE '%caftp%'"
	issues := ValidateSQLStructure(goodSQL, []string{"bm_dc_param", "bm_data_dict"})
	for _, iss := range issues {
		if iss.Type == "error" {
			t.Errorf("有效 SQL 不应有 error: %v", iss.Message)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && search(s, substr)
}

func search(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
