package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	RoleID   int    `json:"role_id"`
	jwt.RegisteredClaims
}

type CreateSpaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	VisitLevel  string `json:"visit_level"`
}

type UpdateSpaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	VisitLevel  string `json:"visit_level"`
}

type CreateDocumentRequest struct {
	ParentID int    `json:"parent_id"`
	SpaceID  int    `json:"space_id"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Content  string `json:"content"`
}

type UpdateDocRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: import_oracle_ddl <ddl_file> [space_name]")
		fmt.Println("Example: import_oracle_ddl databases/importtables/全表DDL.sql")
		fmt.Println("Example: import_oracle_ddl databases/importtables/全表DDL.sql 自定义空间名")
		os.Exit(1)
	}

	ddlFile := os.Args[1]

	jwtSecret, err := os.ReadFile("data/.jwt_secret")
	if err != nil {
		fmt.Printf("读取JWT密钥失败: %v\n", err)
		os.Exit(1)
	}

	token, err := generateToken(strings.TrimSpace(string(jwtSecret)))
	if err != nil {
		fmt.Printf("生成Token失败: %v\n", err)
		os.Exit(1)
	}

	content, err := os.ReadFile(ddlFile)
	if err != nil {
		fmt.Printf("读取DDL文件失败: %v\n", err)
		os.Exit(1)
	}

	tableDDLs, detectedSchema := parseOracleDDL(string(content))

	spaceName := detectedSchema + "数据字典"
	if len(os.Args) >= 3 {
		spaceName = os.Args[2]
	}
	fmt.Printf("解析到 %d 个CREATE TABLE语句\n", len(tableDDLs))

	tableNames := make([]string, 0, len(tableDDLs))
	for name := range tableDDLs {
		tableNames = append(tableNames, name)
	}

	spaceDescription := fmt.Sprintf("数据库表结构DDL")
	spaceID, err := findOrUpdateSpace(token, spaceName, spaceDescription)
	if err != nil {
		fmt.Printf("创建空间失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("空间 ID: %d, 名称: %s\n", spaceID, spaceName)

	parentName := "表结构DDL"
	parentID, _, err := findOrUpdateDocument(token, spaceID, 0, parentName, 2, "")
	if err != nil {
		fmt.Printf("创建父目录失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("父目录 ID: %d, 名称: %s\n", parentID, parentName)

	successCount := 0
	failCount := 0
	createCount := 0
	updateCount := 0

	for i, tableName := range tableNames {
		ddlContent := tableDDLs[tableName]

		var contentBuilder strings.Builder
		contentBuilder.WriteString("# ")
		contentBuilder.WriteString(tableName)
		contentBuilder.WriteString("\n\n```sql\n")
		contentBuilder.WriteString(ddlContent)
		contentBuilder.WriteString("\n```")

		docContent := contentBuilder.String()

		docID, isUpdate, err := findOrUpdateDocument(token, spaceID, parentID, tableName, 1, docContent)
		if err != nil {
			if strings.Contains(err.Error(), "429") {
				fmt.Printf("[%d/%d] 限流，等待10秒后重试: %s\n", i+1, len(tableNames), tableName)
				time.Sleep(10 * time.Second)
				docID, isUpdate, err = findOrUpdateDocument(token, spaceID, parentID, tableName, 1, docContent)
			}
			if err != nil {
				fmt.Printf("[%d/%d] 操作失败: %s - %v\n", i+1, len(tableNames), tableName, err)
				failCount++
				continue
			}
		}

		if isUpdate {
			updateCount++
		} else {
			createCount++
		}
		successCount++

		if (i+1)%20 == 0 {
			fmt.Printf("[%d/%d] 进度: %s (ID: %d)\n", i+1, len(tableNames), tableName, docID)
		}

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Printf("\n完成! 成功: %d, 创建: %d, 更新: %d, 失败: %d, 总计: %d\n", successCount, createCount, updateCount, failCount, len(tableNames))

	fmt.Println("\n正在刷新Text-to-SQL表缓存...")
	if err := refreshText2SQLCache(token); err != nil {
		fmt.Printf("刷新表缓存失败: %v\n", err)
	} else {
		fmt.Println("表缓存刷新完成!")
	}
}

func parseOracleDDL(content string) (map[string]string, string) {
	tables := make(map[string]string)
	detectedSchema := ""

	// (?mi) = multi-line + case-insensitive; capture schema.table branches
	sectionRe := regexp.MustCompile(`(?mi)^-- (\w+)\.(\w+) definition`)
	sections := sectionRe.FindAllStringIndex(content, -1)

	for i, loc := range sections {
		start := loc[0]
		var end int
		if i+1 < len(sections) {
			end = sections[i+1][0]
		} else {
			end = len(content)
		}

		section := content[start:end]
		match := sectionRe.FindStringSubmatch(section)
		if len(match) < 3 {
			continue
		}
		schemaName := strings.ToUpper(match[1])
		tableName := strings.ToUpper(match[2])

		if detectedSchema == "" {
			detectedSchema = schemaName
		}

		tables[tableName] = strings.TrimSpace(section)
	}

	if detectedSchema == "" {
		detectedSchema = "UNKNOWN"
	}

	return tables, detectedSchema
}

func findOrUpdateSpace(token string, spaceName string, description string) (int, error) {
	url := "http://localhost:3001/api/v1/spaces"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("获取空间列表失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return 0, fmt.Errorf("解析空间列表失败: %w", err)
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(apiResp.Data, &result); err != nil {
		return 0, fmt.Errorf("解析空间数据失败: %w", err)
	}

	for _, space := range result.Data {
		if name, ok := space["name"].(string); ok && name == spaceName {
			if id, ok := space["id"].(float64); ok {
				updateReq := UpdateSpaceRequest{
					Name:        spaceName,
					Description: description,
					VisitLevel:  "public",
				}
				reqBody, _ := json.Marshal(updateReq)
				updateURL := fmt.Sprintf("http://localhost:3001/api/v1/spaces/%d", int(id))
				updateReqHTTP, _ := http.NewRequest("PUT", updateURL, bytes.NewReader(reqBody))
				updateReqHTTP.Header.Set("Content-Type", "application/json")
				updateReqHTTP.Header.Set("Authorization", "Bearer "+token)

				updateResp, err := client.Do(updateReqHTTP)
				if err != nil {
					fmt.Printf("[INFO] 更新空间描述失败，继续使用现有空间: %v\n", err)
				} else {
					updateResp.Body.Close()
				}
				return int(id), nil
			}
		}
	}

	createReq := CreateSpaceRequest{
		Name:        spaceName,
		Description: description,
		VisitLevel:  "public",
	}

	reqBody, _ := json.Marshal(createReq)
	url = "http://localhost:3001/api/v1/spaces"
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err = client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("创建空间请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return 0, fmt.Errorf("解析创建空间响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("创建空间失败 (%d): %s", resp.StatusCode, apiResp.Message)
	}

	var space map[string]interface{}
	if err := json.Unmarshal(apiResp.Data, &space); err != nil {
		return 0, fmt.Errorf("解析空间数据失败: %w", err)
	}

	if id, ok := space["id"].(float64); ok {
		return int(id), nil
	}

	return 0, fmt.Errorf("无法获取新创建的空间ID")
}

func findOrUpdateDocument(token string, spaceID int, parentID int, docName string, docType int, content string) (int, bool, error) {
	var existingDocs map[string]int
	if parentID > 0 {
		existingDocs = getExistingDocs(token, parentID)
	} else {
		existingDocs = getRootDocs(token, spaceID)
	}
	if id, exists := existingDocs[docName]; exists {
		err := updateDocument(token, id, UpdateDocRequest{
			Name:    docName,
			Content: content,
		})
		if err != nil {
			return 0, false, fmt.Errorf("更新文档失败: %w", err)
		}
		return id, true, nil
	}

	createReq := CreateDocumentRequest{
		ParentID: parentID,
		SpaceID:  spaceID,
		Name:     docName,
		Type:     docType,
		Content:  content,
	}

	reqBody, _ := json.Marshal(createReq)
	url := "http://localhost:3001/api/v1/documents"
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, false, fmt.Errorf("创建文档请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return 0, false, fmt.Errorf("解析创建文档响应失败: %w", err)
	}

	if resp.StatusCode == http.StatusConflict {
		id, err := searchDocumentByName(token, spaceID, parentID, docName)
		if err != nil {
			var retryDocs map[string]int
			if parentID > 0 {
				retryDocs = getExistingDocs(token, parentID)
			} else {
				retryDocs = getRootDocs(token, spaceID)
			}
			if cid, exists := retryDocs[docName]; exists {
				id = cid
			} else {
				return 0, false, fmt.Errorf("创建文档失败 (409): %s，重试后仍未找到", apiResp.Message)
			}
		}
		updateErr := updateDocument(token, id, UpdateDocRequest{
			Name:    docName,
			Content: content,
		})
		if updateErr != nil {
			return 0, false, fmt.Errorf("更新冲突文档失败: %w", updateErr)
		}
		return id, true, nil
	}
	if resp.StatusCode != http.StatusCreated {
		return 0, false, fmt.Errorf("创建文档失败 (%d): %s", resp.StatusCode, apiResp.Message)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(apiResp.Data, &doc); err != nil {
		return 0, false, fmt.Errorf("解析文档数据失败: %w", err)
	}

	if id, ok := doc["id"].(float64); ok {
		return int(id), false, nil
	}

	return 0, false, fmt.Errorf("无法获取新创建的文档ID")
}

func searchDocumentByName(token string, spaceID int, parentID int, docName string) (int, error) {
	url := fmt.Sprintf("http://localhost:3001/api/v1/documents/search?space_id=%d&keyword=%s", spaceID, docName)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var apiResp APIResponse
		if json.Unmarshal(body, &apiResp) == nil {
			var results []map[string]interface{}
			if json.Unmarshal(apiResp.Data, &results) == nil {
				for _, r := range results {
					if name, ok := r["name"].(string); ok && name == docName {
						pid, _ := r["parent_id"].(float64)
						if (parentID == 0 && pid == 0) || (parentID > 0 && int(pid) == parentID) {
							if id, ok := r["id"].(float64); ok {
								return int(id), nil
							}
						}
					}
				}
			}
		}
	}
	if resp != nil {
		resp.Body.Close()
	}

	treeURL := fmt.Sprintf("http://localhost:3001/api/v1/documents/tree?space_id=%d", spaceID)
	treeReq, _ := http.NewRequest("GET", treeURL, nil)
	treeReq.Header.Set("Authorization", "Bearer "+token)

	treeResp, err := client.Do(treeReq)
	if err == nil && treeResp.StatusCode == http.StatusOK {
		treeBody, _ := io.ReadAll(treeResp.Body)
		treeResp.Body.Close()
		var treeApiResp APIResponse
		if json.Unmarshal(treeBody, &treeApiResp) == nil {
			var nodes []map[string]interface{}
			if json.Unmarshal(treeApiResp.Data, &nodes) == nil {
				id := findInTree(nodes, parentID, docName)
				if id > 0 {
					return id, nil
				}
			}
		}
	}
	if treeResp != nil {
		treeResp.Body.Close()
	}

	return 0, fmt.Errorf("未找到 '%s'", docName)
}

func findInTree(nodes []map[string]interface{}, targetParentID int, targetName string) int {
	for _, node := range nodes {
		name, _ := node["name"].(string)
		pid, _ := node["parent_id"].(float64)
		id, _ := node["id"].(float64)

		if name == targetName && int(pid) == targetParentID {
			return int(id)
		}

		if children, ok := node["children"].([]map[string]interface{}); ok {
			if foundID := findInTree(children, targetParentID, targetName); foundID > 0 {
				return foundID
			}
		}
	}
	return 0
}

func getExistingDocs(token string, parentID int) map[string]int {
	docs := make(map[string]int)

	url := fmt.Sprintf("http://localhost:3001/api/v1/documents/%d/children", parentID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[WARN] 获取子文档失败(parent=%d): %v\n", parentID, err)
		return docs
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[WARN] 获取子文档返回 %d(parent=%d): %s\n", resp.StatusCode, parentID, string(body))
		return docs
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Printf("[WARN] 解析子文档响应失败(parent=%d): %v, body=%s\n", parentID, err, string(body))
		return docs
	}

	var children []map[string]interface{}
	if err := json.Unmarshal(apiResp.Data, &children); err != nil {
		fmt.Printf("[WARN] 解析子文档数据失败(parent=%d): %v, raw=%s\n", parentID, err, string(apiResp.Data))
		return docs
	}

	for _, child := range children {
		if name, ok := child["name"].(string); ok {
			if id, ok := child["id"].(float64); ok {
				docs[name] = int(id)
			}
		}
	}

	return docs
}

func getRootDocs(token string, spaceID int) map[string]int {
	docs := make(map[string]int)

	url := fmt.Sprintf("http://localhost:3001/api/v1/documents/roots?space_id=%d", spaceID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[ERROR] 获取根文档失败(space=%d): %v\n", spaceID, err)
		return docs
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[ERROR] 获取根文档返回 %d (space=%d)\n", resp.StatusCode, spaceID)
		if resp.StatusCode == 403 {
			fmt.Printf("[ERROR] 403 Forbidden - 可能原因:\n")
			fmt.Printf("  1. 空间 %d 是 member-only，token 未被解析\n", spaceID)
			fmt.Printf("  2. 后端公开路由未添加 OptionalAuthMiddleware\n")
			fmt.Printf("  3. 后端服务未重启应用最新配置\n")
		}
		fmt.Printf("[ERROR] 响应内容: %s\n", string(body))
		return docs
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Printf("[WARN] 解析根文档响应失败(space=%d): %v, body=%s\n", spaceID, err, string(body))
		return docs
	}

	var roots []map[string]interface{}
	if err := json.Unmarshal(apiResp.Data, &roots); err != nil {
		fmt.Printf("[WARN] 解析根文档数据失败(space=%d): %v, raw=%s\n", spaceID, err, string(apiResp.Data))
		return docs
	}

	for _, root := range roots {
		if name, ok := root["name"].(string); ok {
			if id, ok := root["id"].(float64); ok {
				docs[name] = int(id)
			}
		}
	}

	return docs
}

func generateToken(secret string) (string, error) {
	claims := Claims{
		UserID:   1,
		Username: "admin",
		RoleID:   1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "citebase",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func updateDocument(token string, docID int, req UpdateDocRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	url := fmt.Sprintf("http://localhost:3001/api/v1/documents/%d", docID)
	httpReq, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("解析响应失败: %s", string(respBody))
	}

	if resp.StatusCode == 429 {
		return fmt.Errorf("429 请求过于频繁")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API错误 (%d): %s", resp.StatusCode, apiResp.Message)
	}

	return nil
}

func createDocument(token string, req CreateDocumentRequest) (int, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := "http://localhost:3001/api/v1/documents"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return 0, fmt.Errorf("解析响应失败: %s", string(respBody))
	}

	if resp.StatusCode == 429 {
		return 0, fmt.Errorf("429 请求过于频繁")
	}
	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("API错误 (%d): %s", resp.StatusCode, apiResp.Message)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(apiResp.Data, &doc); err != nil {
		return 0, fmt.Errorf("解析文档数据失败: %w", err)
	}

	if id, ok := doc["id"].(float64); ok {
		return int(id), nil
	}

	return 0, fmt.Errorf("无法获取文档ID")
}

func refreshText2SQLCache(token string) error {
	url := "http://localhost:3001/api/v1/ai/data-query/refresh"
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析响应失败: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API错误 (%d): %s", resp.StatusCode, apiResp.Message)
	}

	return nil
}
