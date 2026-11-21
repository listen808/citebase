package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 这台门禁管的是"后端说给人听的那句话，前端能不能翻成界面语言"。
//
// 起因（2026-10-11 实测）：245 处把中文直接塞进响应封套的 message 字段，散在 37 个文件里
// （去重后 123 条），而前端只有 err.message 一根管子 —— 报错页、toast、登录页错误位读的都是它。
// 于是英文界面的用户看到一句中文，且**前端修不了**：那句话不是前端产的字符串，
// 换任何语言包都盖不住。这是 i18n 十四批够不到的那一类欠账。
//
// 现在的形状：internal/response/messages.go 那张表（稳定键 + 中文）是唯一目录，
// 响应多带一个 message_key；前端在 ApiError 构造处按当前语言替换（web/src/api/client.ts），
// 所以两百多个显示 err.message 的调用点不用改就跟着变。
// 英文住在 web/scripts/server-messages.en.json，前端语言包 src/i18n/locales/server-messages.ts 是生成物。
//
// 四条判据各挡一种复发：
//   ① 键集是契约：只增不改不删 —— 改名的表现不是报错，是"英文界面里零星几句中文"；
//   ② 每个登记键都要有出口在用 —— 没人发的键是下一份 ConfigKeyMainTitle 式的死代码；
//   ③ 中文与英文都不许含插值 —— 封套里没有参数位，"%s" 会原样印到界面上；
//   ④ 未迁移的中文直出只许减少，减少了必须把基线一起降下来。
// 另有一条行为判据住在 internal/response/messages_test.go：出口发出去的 message
// 必须逐字等于登记值、message_key 必须等于键 —— 那才是"表与真实话术没分叉"的证明。
//
// 反过来说这台门禁自己也翻过一次车：原第五判据"登记值必须能在源码里找到原句"是**自证式**的 ——
// 它拿包含 messages.go 的源码合集去找 messages.go 里的内容，永远命中（迁移后中文只住在表里，
// 更是不可能失败）。发现方式是把登记值改一个字，门禁纹丝不动。判据见 internalCallSites。
//
// 反向验证：改一个键名（①红）、把某条键的出口引用注释掉（②红）、往文案里塞 %d（③红）、
// 往出口加一句没登记的中文（④红）、把登记值改一个字（messages_test 红）。

// serverMessageKeyBaseline 是 2026-10-11 收口那一刻的键集。
// 加新键：往这里加一行。删键或改键名：必须同时在这里改掉并写出理由 ——
// 老前端读到没登记的键会退回服务端中文，不是崩，但语言错了，而且没人会报。
var serverMessageKeyBaseline = []string{
	"account_locked",
	"action_too_frequent",
	"active_field_required",
	"ai_adaptive_retriever_unconfigured",
	"ai_analyzer_unconfigured",
	"ai_chat_failed",
	"ai_cron_unconfigured",
	"ai_email_writer_unconfigured",
	"ai_embedding_unconfigured",
	"ai_generate_failed",
	"ai_model_unconfigured",
	"ai_primary_model_unconfigured",
	"ai_quality_unconfigured",
	"ai_ragas_uninitialized",
	"ai_recommend_unconfigured",
	"ai_report_writer_unconfigured",
	"ai_retriever_unconfigured",
	"ai_service_disabled",
	"ai_safety_blocked",
	"ai_service_unconfigured",
	"ai_table_writer_unconfigured",
	"ai_translator_unconfigured",
	"already_initialized",
	"attachment_not_found",
	"audio_field_missing",
	"audio_open_failed",
	"audio_read_failed",
	"audit_record_not_found",
	"auth_failed",
	"concurrent_init",
	"confluence_convert_failed",
	"contact_fields_empty",
	"contact_not_found",
	"conversation_not_found",
	"credentials_empty",
	"data_source_connect_failed",
	"data_source_not_found",
	"doc_ids_required",
	"doc_modified_by_other",
	"doc_not_found",
	"email_server_name_taken",
	"explanation_parse_failed",
	"file_too_large",
	"history_id_invalid",
	"history_not_found",
	"history_query_failed",
	"host_header_unsafe_embed",
	"host_header_unsafe_page",
	"host_header_unsafe_script",
	"id_invalid",
	"id_param_invalid",
	"internal_error",
	"invalid_attachment_id",
	"invalid_credentials",
	"invalid_data_source_id",
	"invalid_doc_id",
	"invalid_numeric_id",
	"invalid_parameter",
	"invalid_source_kind",
	"link_fields_empty",
	"link_id_invalid",
	"link_name_taken",
	"link_not_found",
	"login_failed",
	"memory_create_failed",
	"memory_delete_failed",
	"memory_list_failed",
	"memory_update_failed",
	"memory_recall_failed",
	"memory_search_failed",
	"memory_stats_failed",
	"no_contact_chosen",
	"no_file_chosen",
	"no_file_uploaded",
	"no_link_chosen",
	"no_permission_analyze_space",
	"no_permission_compare_versions",
	"no_permission_conversation",
	"no_permission_conversation_delete",
	"no_permission_conversation_update",
	"no_permission_delete_attachment",
	"no_permission_document_comments",
	"no_permission_document_space",
	"no_permission_export_document",
	"no_permission_historical_document",
	"no_permission_kb_chunks",
	"no_permission_kb_recall_test",
	"no_permission_kb_stats",
	"no_permission_kbs_space",
	"no_permission_read_attachment",
	"no_permission_space",
	"no_permission_space_content",
	"no_permission_space_graph",
	"no_permission_sql_history",
	"no_permission_sql_history_read",
	"no_permission_template_edit",
	"no_permission_upload_attachment",
	"no_permission_version_history",
	"not_signed_in",
	"notion_convert_failed",
	"openapi_users_module_denied",
	"page_not_found",
	"page_updated",
	"page_version_not_found",
	"parameter_error",
	"plugin_key_taken",
	"question_required",
	"record_not_found",
	"refresh_too_frequent",
	"request_body_unreadable",
	"request_parameter_error",
	"resource_not_found",
	"restore_needs_confirm",
	"signed_out_or_token_invalid",
	"space_disallows_export",
	"space_disallows_share",
	"space_id_and_slug_required",
	"space_id_and_source_doc_required",
	"space_id_invalid",
	"space_id_required",
	"space_name_illegal_chars",
	"space_name_illegal_chars_hint",
	"space_name_taken",
	"space_page_claim_required",
	"sso_auth_code_invalid",
	"sso_name_taken",
	"sso_not_found",
	"sso_prefix_taken",
	"text_empty",
	"too_many_requests",
	"topic_required",
	"url_required",
	"wiki_no_compilable_content",
	"workflow_name_empty",
	"workflow_not_found",
	"wrong_share_password",
}

// 两档基线，都只许减少；减少了却不跟着改数字同样判红（基线当装饰比没有基线更坏）。
//
// serverMessageUnmigratedBaseline：把中文直接写进 response.Xxx(c, "中文") 的调用点。
// 批次十六B 把最后 5 处拼接型也迁完了（细节改走 message_detail，见 keyed.go），所以这一档归 0 ——
// 从此这条判红的意思是"有人新写了一句没登记的中文"。
//
// serverMessageFieldBaseline：**另一档，形状不同**：中文写在数据字段里
// （`gin.H{"message": "AI Provider 未配置"}` 这一族），封套的 message_key 帮不上忙，
// 前端通常直接把该字段显示出来。批次十六B 现场量到 42 处 / 20 个文件。
// 这一档还没迁，先把数钉住别让它长，迁一条降一条。
const (
	serverMessageUnmigratedBaseline = 0
	serverMessageFieldBaseline      = 44
)

func TestServerMessageKeysAreAContract(t *testing.T) {
	zh := serverMessageRegistry(t)
	baseline := map[string]bool{}
	for _, k := range serverMessageKeyBaseline {
		baseline[k] = true
	}

	var lost []string
	for _, k := range serverMessageKeyBaseline {
		if _, ok := zh[k]; !ok {
			lost = append(lost, k)
		}
	}
	if len(lost) > 0 {
		sort.Strings(lost)
		t.Errorf("键集少了 %d 个（契约只增不改不删）：%s", len(lost), strings.Join(lost, ", "))
	}

	var added []string
	for k := range zh {
		if !baseline[k] {
			added = append(added, k)
		}
	}
	if len(added) > 0 && !serverMessageBaselineAllowsAdded(t, added) {
		sort.Strings(added)
		t.Errorf("新增了 %d 个键但没写进 serverMessageKeyBaseline：%s —— 键集是基线，加了就要登记", len(added), strings.Join(added, ", "))
	}
}

// serverMessageBaselineAllowsAdded 给"新增键"一条正路：默认判红，除非调用方在
// serverMessageBaselineAdditions 里为该键写了一句理由。这样"加键"必须留下意图，
// 而不是顺手多一条没人核对的契约。
func serverMessageBaselineAllowsAdded(t *testing.T, added []string) bool {
	for _, k := range added {
		if _, ok := serverMessageBaselineAdditions[k]; !ok {
			return false
		}
	}
	return len(added) > 0
}

// serverMessageBaselineAdditions 是新键的理由登记处（当前为空 = 还没人加过）。
var serverMessageBaselineAdditions = map[string]string{
	// fulltext_search_disabled：系统设置 search 组「开启全文搜索」总开关的拒绝文案，
	// 出口在 SearchAPI.SearchDocuments（2026-10-19 摆设键接线批次）。
	"fulltext_search_disabled": "系统设置「开启全文搜索」关闭时 /search/fulltext 的 503 拒绝文案",
}

func TestEveryServerMessageIsSent(t *testing.T) {
	// 找的是**常量名**出现在 messages.go 之外的地方。
	// 上一版找的是 `"slug"`，而 slug 就定义在 messages.go 里， sources 又包含 messages.go ——
	// 于是这条判据自证式地永远命中（键没被任何出口用，它照样绿）。
	// 迁移之后中文也只住在 messages.go，所以"有没有人发这句话"只能看常量有没有被引用。
	zh := serverMessageRegistry(t)
	sites := internalCallSites(t)
	names := serverMessageConstNames(t)

	var dead []string
	for slug, text := range zh {
		name := names[slug]
		if name == "" {
			dead = append(dead, slug+"（找不到对应常量）")
			continue
		}
		if !strings.Contains(sites, name) {
			dead = append(dead, slug+"（"+text+"）")
		}
	}
	if len(dead) > 0 {
		sort.Strings(dead)
		t.Errorf("%d 个登记键没有任何出口引用（登记了却不发 = 死表）：\n  %s", len(dead), strings.Join(dead, "\n  "))
	}
}

func TestServerMessagesCarryNoInterpolation(t *testing.T) {
	verb := regexp.MustCompile(`%[sdv]|` + `\{\w+\}`)
	check := func(from string, m map[string]string) {
		for slug, text := range m {
			if verb.MatchString(text) {
				t.Errorf("%s 键 %q 的文案含插值：%q —— 封套没有参数位，占位符会原样印到界面上", from, slug, text)
			}
		}
	}
	check("中文", serverMessageRegistry(t))
	check("英文", serverMessageEnglish(t))
}

// TestServerMessageTextMatchesSource 曾经存在，后来删掉了，理由是它测不到东西：
// 它拿登记值去 internal 的源码里找"原句"，而迁移之后那句话**只**住在 messages.go 里 ——
// 搜索范围包含 messages.go 自己，所以永远命中。真正该测的是"出口发出去的那句是不是登记值"，
// 那是行为，住在 internal/response/messages_test.go（httptest 直接看响应体）。

func TestUnmigratedChineseOnlyDecreases(t *testing.T) {
	sources := internalGoSources(t)
	n := len(unmigratedResponseRe.FindAllString(sources, -1))
	switch {
	case n > serverMessageUnmigratedBaseline:
		t.Errorf("没登记的中文直出从 %d 涨到 %d —— 新代码请用带键出口：response.XxxKey(c, response.MsgYyy)",
			serverMessageUnmigratedBaseline, n)
	case n < serverMessageUnmigratedBaseline:
		t.Errorf("没登记的中文直出降到 %d，请把 serverMessageUnmigratedBaseline 一起改下来", n)
	}
}

// 数据字段里那档：形状与封套不同，所以分开数、分开降。
func TestChineseInDataFieldsOnlyDecreases(t *testing.T) {
	sources := internalGoSources(t)
	hits := chineseFieldRe.FindAllString(sources, -1)
	switch {
	case len(hits) > serverMessageFieldBaseline:
		lines := make([]string, 0, len(hits))
		for _, h := range hits {
			if len(h) > 90 {
				h = h[:90]
			}
			lines = append(lines, h)
		}
		t.Errorf("写在数据字段里的中文从 %d 涨到 %d —— 这一档还没有键，先别再长：\n  %s",
			serverMessageFieldBaseline, len(hits), strings.Join(lines, "\n  "))
	case len(hits) < serverMessageFieldBaseline:
		t.Errorf("数据字段里的中文降到 %d，请把 serverMessageFieldBaseline 一起改下来", len(hits))
	}
}

// ---------- 读上游 ----------

var (
	registryConstRe = regexp.MustCompile("(?m)^\t(Msg\\w+)\\s*=\\s*\"([^\"]+)\"")
	registryTextRe  = regexp.MustCompile("(?m)^\t(Msg\\w+):\\s+(`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"),")
	enEntryRe       = regexp.MustCompile(`"(a-z0-9_)+":\s*"`)
)

func serverMessageRegistry(t *testing.T) map[string]string {
	t.Helper()
	src := readRepoFile(t, "internal/response/messages.go")

	slugOf := map[string]string{}
	for _, m := range registryConstRe.FindAllStringSubmatch(src, -1) {
		slugOf[m[1]] = m[2]
	}
	out := map[string]string{}
	for _, m := range registryTextRe.FindAllStringSubmatch(src, -1) {
		slug, ok := slugOf[m[1]]
		if !ok {
			t.Errorf("messages.go 有登记文案但没有常量：%s", m[1])
			continue
		}
		text, err := strconv.Unquote(m[2])
		if err != nil {
			t.Errorf("解析 %s 的登记值失败：%v", m[1], err)
			continue
		}
		out[slug] = text
	}
	// 下界：一条都没解析出来就是判据自己坏了，此时的通过不可信
	if len(out) < 100 {
		t.Fatalf("只从 messages.go 解析出 %d 条（现行 122）—— 判据坏了", len(out))
	}
	if len(out) != len(slugOf) {
		t.Errorf("常量 %d 个 / 登记文案 %d 个，数量不等 —— 要么漏登记，要么正则漏匹配", len(slugOf), len(out))
	}
	return out
}

func serverMessageEnglish(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "web", "scripts", "server-messages.en.json"))
	if err != nil {
		t.Fatalf("读 web/scripts/server-messages.en.json: %v", err)
	}
	// 这张表是扁平的 "slug": "text"，逐行解析就够，不必为测试引第三方 JSON。
	out := map[string]string{}
	linesRe := regexp.MustCompile(`^\s{2}"([a-z0-9_]+)":\s*"((?:[^"\\]|\\.)*)",?\s*$`)
	for _, line := range strings.Split(string(data), "\n") {
		m := linesRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		text, err := strconv.Unquote(`"` + m[2] + `"`)
		if err != nil {
			t.Errorf("解析英文 %s 失败：%v", m[1], err)
			continue
		}
		out[m[1]] = text
	}
	if len(out) < 100 {
		t.Fatalf("只解析出 %d 条英文文案 —— 判据坏了", len(out))
	}

	zh := serverMessageRegistry(t)
	var missing, stale []string
	for slug := range zh {
		if _, ok := out[slug]; !ok {
			missing = append(missing, slug)
		}
	}
	for slug := range out {
		if _, ok := zh[slug]; !ok {
			stale = append(stale, slug)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("英文表缺 %d 个键（前端查不到译文会静默退回中文）：%s", len(missing), strings.Join(missing, ", "))
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("英文表有 %d 个后端不认的键：%s", len(stale), strings.Join(stale, ", "))
	}
	return out
}

var internalSourceCache string

func internalGoSources(t *testing.T) string {
	t.Helper()
	if internalSourceCache != "" {
		return internalSourceCache
	}
	var sb strings.Builder
	files := 0
	err := filepath.Walk(filepath.Join("..", "..", "internal"), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files++
		sb.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 internal: %v", err)
	}
	if files < 500 {
		t.Fatalf("只读了 %d 个非测试 .go —— 遍历判据坏了，此时的计数不可信", files)
	}
	internalSourceCache = sb.String()
	return internalSourceCache
}

// internalCallSites 是"出口代码"，不含 messages.go 本身。
// 判"某个键有没有人发"必须用它 —— 拿包含登记表自身的源码去找登记表里的东西，
// 是一条会永远命中的自证判据（上一版就犯了这个错）。
func internalCallSites(t *testing.T) string {
	t.Helper()
	src := readRepoFile(t, "internal/response/messages.go")
	sites := strings.Replace(internalGoSources(t), src, "", 1)
	if sites == internalGoSources(t) {
		t.Fatal("没能从源码合集里剔掉 messages.go —— 路径或内容变了，这条判据会退回自证式")
	}
	return sites
}

// serverMessageConstNames 反过来建 slug → MsgXxx 的表，供"有没有人发"用。
func serverMessageConstNames(t *testing.T) map[string]string {
	t.Helper()
	src := readRepoFile(t, "internal/response/messages.go")
	out := map[string]string{}
	for _, m := range registryConstRe.FindAllStringSubmatch(src, -1) {
		out[m[2]] = m[1]
	}
	return out
}

// 没登记的中文直出（response.Xxx(c, "中文") 这一族）。
var unmigratedResponseRe = regexp.MustCompile(
	`response\.(BadRequest|Unauthorized|Forbidden|Conflict|NotFound|ServerError|SuccessWithMsg|TooManyRequests)\(c,\s*"(?:[^"\\]|\\.)*\p{Han}`)

// 另一档：中文写在**数据字段**里（gin.H{"message": "..."}），封套的键帮不上忙。
var chineseFieldRe = regexp.MustCompile(
	`"(?:message|msg|error|title|content|reason)"\s*:\s*"(?:[^"\\]|\\.)*\p{Han}`)
