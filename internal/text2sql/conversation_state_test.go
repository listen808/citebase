package text2sql

import (
	"strings"
	"testing"
)

func TestNewConversationState(t *testing.T) {
	cs := NewConversationState(1, 100)
	if cs == nil {
		t.Fatal("NewConversationState returned nil")
	}
	if cs.ConversationID != 1 {
		t.Errorf("ConversationID = %d, want 1", cs.ConversationID)
	}
	if cs.SpaceID != 100 {
		t.Errorf("SpaceID = %d, want 100", cs.SpaceID)
	}
	if cs.UsedTables == nil {
		t.Error("UsedTables should be initialized")
	}
	if cs.SQLHistory == nil {
		t.Error("SQLHistory should be initialized")
	}
}

func TestRecordTurn(t *testing.T) {
	cs := NewConversationState(1, 100)
	cs.RecordTurn("查询员工", "SELECT * FROM employees", nil)
	if len(cs.SQLHistory) != 1 {
		t.Fatalf("SQLHistory length = %d, want 1", len(cs.SQLHistory))
	}
	if cs.SQLHistory[0].UserQuery != "查询员工" {
		t.Errorf("UserQuery = %q, want %q", cs.SQLHistory[0].UserQuery, "查询员工")
	}
	if cs.CurrentSQL != "SELECT * FROM employees" {
		t.Errorf("CurrentSQL = %q, want %q", cs.CurrentSQL, "SELECT * FROM employees")
	}
}

func TestRecordTurnMultiple(t *testing.T) {
	cs := NewConversationState(1, 100)
	cs.RecordTurn("查询员工", "SELECT * FROM employees", nil)
	cs.RecordTurn("按部门分组", "SELECT dept, COUNT(*) FROM employees GROUP BY dept", nil)
	if len(cs.SQLHistory) != 2 {
		t.Fatalf("SQLHistory length = %d, want 2", len(cs.SQLHistory))
	}
	if cs.CurrentSQL != "SELECT dept, COUNT(*) FROM employees GROUP BY dept" {
		t.Errorf("CurrentSQL not updated to latest")
	}
	if cs.SQLHistory[1].TurnID != 2 {
		t.Errorf("TurnID = %d, want 2", cs.SQLHistory[1].TurnID)
	}
}

// 任务2：意图检测与引用解析测试

func TestDetectIntent_NewQuery(t *testing.T) {
	cs := NewConversationState(1, 100)
	cs.RecordTurn("查询员工", "SELECT * FROM employees", nil)

	intent := cs.DetectIntent("查询所有部门的名称")
	if intent.IntentType != "new_query" {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, "new_query")
	}
}

func TestDetectIntent_Modify(t *testing.T) {
	cs := NewConversationState(1, 100)
	cs.RecordTurn("查询员工", "SELECT * FROM employees", nil)

	intent := cs.DetectIntent("把WHERE条件改成部门为技术部")
	if intent.IntentType != "modify" {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, "modify")
	}
}

func TestDetectIntent_FollowUp(t *testing.T) {
	cs := NewConversationState(1, 100)
	cs.RecordTurn("查询员工", "SELECT * FROM employees", nil)

	intent := cs.DetectIntent("按入职时间排序")
	if intent.IntentType != "follow_up" {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, "follow_up")
	}
}

func TestDetectIntent_Clarify(t *testing.T) {
	cs := NewConversationState(1, 100)
	cs.RecordTurn("查询员工", "SELECT * FROM employees", nil)

	intent := cs.DetectIntent("我说的是上个月的数据")
	if intent.IntentType != "clarify" {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, "clarify")
	}
}

func TestResolveReference_LastQuery(t *testing.T) {
	cs := NewConversationState(1, 100)
	cs.RecordTurn("查询员工", "SELECT * FROM employees", nil)

	resolved := cs.ResolveReference("刚才那个查询加上时间筛选")
	if !strings.Contains(resolved, "SELECT * FROM employees") {
		t.Errorf("ResolveReference should include last SQL, got %q", resolved)
	}
}

func TestResolveReference_NoHistory(t *testing.T) {
	cs := NewConversationState(1, 100)
	resolved := cs.ResolveReference("查询员工")
	if resolved != "查询员工" {
		t.Errorf("ResolveReference with no history should return original, got %q", resolved)
	}
}

// shouldBypassClarification 测试
func TestShouldBypassClarification(t *testing.T) {
	tests := []struct {
		name     string
		question string
		history  []ConversationMessage
		want     bool
	}{
		// 规则 1：用户刚刚回答了澄清
		{
			name:     "我选择前缀",
			question: "我选择基于update_time",
			history:  nil,
			want:     true,
		},
		{
			name:     "我选前缀",
			question: "我选创建时间",
			history:  nil,
			want:     true,
		},
		// 规则 2：前端包装的"已选择"标记
		{
			name:     "已选择标记",
			question: "已选择：基于update_time",
			history:  nil,
			want:     true,
		},
		// 规则 3：明确的时间字段引用
		{
			name:     "基于update_time",
			question: "基于update_time查询",
			history:  nil,
			want:     true,
		},
		{
			name:     "基于创建时间",
			question: "基于创建时间查询",
			history:  nil,
			want:     true,
		},
		// 规则 6：用户明确确认表名
		{
			name:     "表正确",
			question: "twb_send_sms表正确",
			history:  nil,
			want:     true,
		},
		{
			name:     "表确认",
			question: "twb_send_sms表确认",
			history:  nil,
			want:     true,
		},
		{
			name:     "表在空间里",
			question: "twb_send_sms表在空间里",
			history:  nil,
			want:     true,
		},
		{
			name:     "表没问题",
			question: "twb_send_sms表没问题",
			history:  nil,
			want:     true,
		},
		// 规则 7：用户已经澄清了3次以上
		{
			name:     "澄清3次以上",
			question: "随便什么问题",
			history: []ConversationMessage{
				{Role: "assistant", Content: "我需要先确认几个信息，才能准确生成查询："},
				{Role: "user", Content: "表名正确"},
				{Role: "assistant", Content: "我需要先确认几个信息，才能准确生成查询："},
				{Role: "user", Content: "字段正确"},
				{Role: "assistant", Content: "我需要先确认几个信息，才能准确生成查询："},
				{Role: "user", Content: "条件正确"},
			},
			want: true,
		},
		// 不应该放行的情况
		{
			name:     "普通问题",
			question: "查询员工信息",
			history:  nil,
			want:     false,
		},
		{
			name:     "澄清未达3次",
			question: "随便什么问题",
			history: []ConversationMessage{
				{Role: "assistant", Content: "我需要先确认几个信息，才能准确生成查询："},
				{Role: "user", Content: "表名正确"},
				{Role: "assistant", Content: "我需要先确认几个信息，才能准确生成查询："},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldBypassClarification(tt.question, tt.history, nil)
			if got != tt.want {
				t.Errorf("shouldBypassClarification(%q) = %v, want %v", tt.question, got, tt.want)
			}
		})
	}
}
