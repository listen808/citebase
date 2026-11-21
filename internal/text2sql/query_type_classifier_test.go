package text2sql

import "testing"

func TestInferSQLQueryType_Basic(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		expected QueryType
	}{
		{"empty string", "", QueryTypeUnknown},
		{"simple select", "SELECT * FROM users", QueryTypeBasic},
		{"select with where", "SELECT name, age FROM users WHERE age > 18", QueryTypeBasic},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := inferSQLQueryType(tc.sql)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestInferSQLQueryType_Aggregation(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"count", "SELECT COUNT(*) FROM orders"},
		{"sum", "SELECT SUM(amount) FROM orders"},
		{"avg", "SELECT AVG(price) FROM products"},
		{"max", "SELECT MAX(created_at) FROM users"},
		{"min", "SELECT MIN(price) FROM products"},
		{"group by", "SELECT category, COUNT(*) FROM products GROUP BY category"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := inferSQLQueryType(tc.sql)
			if result != QueryTypeAggregation {
				t.Errorf("expected QueryTypeAggregation, got %v for: %s", result, tc.sql)
			}
		})
	}
}

func TestInferSQLQueryType_MultiJoin(t *testing.T) {
	sql := "SELECT u.name, o.total FROM users u JOIN orders o ON u.id = o.user_id"
	result := inferSQLQueryType(sql)
	if result != QueryTypeMultiJoin {
		t.Errorf("expected QueryTypeMultiJoin, got %v", result)
	}
}

func TestInferSQLQueryType_Subquery(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"in select", "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)"},
		{"exists", "SELECT * FROM users WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)"},
		{"in no space", "SELECT * FROM users WHERE id IN(SELECT user_id FROM orders)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := inferSQLQueryType(tc.sql)
			if result != QueryTypeSubquery {
				t.Errorf("expected QueryTypeSubquery, got %v for: %s", result, tc.sql)
			}
		})
	}
}

func TestInferSQLQueryType_CTE(t *testing.T) {
	sql := "WITH active_users AS (SELECT * FROM users WHERE status = 'active') SELECT * FROM active_users"
	result := inferSQLQueryType(sql)
	if result != QueryTypeCTE {
		t.Errorf("expected QueryTypeCTE, got %v", result)
	}
}

func TestInferSQLQueryType_CTE_WithNoData_Excluded(t *testing.T) {
	sql := "CREATE TABLE foo WITH NO DATA"
	result := inferSQLQueryType(sql)
	if result == QueryTypeCTE {
		t.Errorf("WITH NO DATA should not be classified as CTE, got %v", result)
	}
}

func TestInferSQLQueryType_ComplexReport(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"row_number", "SELECT ROW_NUMBER() OVER (ORDER BY score) rank, name FROM students"},
		{"rank", "SELECT RANK() OVER (PARTITION BY class ORDER BY score) rk FROM students"},
		{"dense_rank", "SELECT DENSE_RANK() OVER (ORDER BY score) FROM students"},
		{"ntile", "SELECT NTILE(4) OVER (ORDER BY score) FROM students"},
		{"over with space", "SELECT SUM(amount) OVER (ORDER BY date) FROM orders"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := inferSQLQueryType(tc.sql)
			if result != QueryTypeComplexReport {
				t.Errorf("expected QueryTypeComplexReport, got %v for: %s", result, tc.sql)
			}
		})
	}
}

func TestInferSQLQueryType_CaseInsensitive(t *testing.T) {
	sql := "select count(*) from orders group by category"
	result := inferSQLQueryType(sql)
	if result != QueryTypeAggregation {
		t.Errorf("expected QueryTypeAggregation (case-insensitive), got %v", result)
	}
}
