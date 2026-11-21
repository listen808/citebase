package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/log"
	"strings"
	"time"
	"unicode"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

type CanonicalEntity struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	SourceDocIDs []int    `json:"source_doc_ids"`
	Aliases      []string `json:"aliases"`
	Confidence   string   `json:"confidence"`
}

type EntityResolver struct {
	provider provider.Provider
}

func NewEntityResolver(p provider.Provider) *EntityResolver {
	return &EntityResolver{provider: p}
}

func (er *EntityResolver) ResolveEntities(ctx context.Context, summaries []DocSummary) []CanonicalEntity {
	if len(summaries) == 0 {
		return nil
	}

	allAliases := er.collectAliases(summaries)

	exactGroups := er.exactMatch(allAliases)

	resolvedMap := er.fuzzyMatch(exactGroups)

	var resolved []CanonicalEntity
	for _, group := range resolvedMap {
		resolved = append(resolved, group)
	}

	if er.provider != nil && len(resolved) >= 2 {
		resolved = er.llmResolve(ctx, resolved)
	}

	return resolved
}

type aliasEntry struct {
	original string
	aliases  []string
	docID    int
}

func (er *EntityResolver) collectAliases(summaries []DocSummary) []aliasEntry {
	seen := make(map[int]bool)
	var entries []aliasEntry
	for _, s := range summaries {
		if seen[s.SourceDocID] {
			continue
		}
		seen[s.SourceDocID] = true

		entry := aliasEntry{
			original: s.SourceName,
			aliases:  s.Aliases,
			docID:    s.SourceDocID,
		}
		if len(entry.aliases) == 0 {
			entry.aliases = []string{}
		}
		entries = append(entries, entry)

		for _, url := range s.URLs {
			urlEntry := aliasEntry{
				original: url.Label,
				aliases:  []string{url.URL},
				docID:    s.SourceDocID,
			}
			if urlEntry.original != "" {
				entries = append(entries, urlEntry)
			}
		}

		for _, item := range s.ConfigItems {
			itemEntry := aliasEntry{
				original: item.Key,
				aliases:  []string{item.Value},
				docID:    s.SourceDocID,
			}
			if itemEntry.original != "" {
				entries = append(entries, itemEntry)
			}
		}
	}
	return entries
}

func (er *EntityResolver) exactMatch(entries []aliasEntry) map[string]CanonicalEntity {
	groups := make(map[string]CanonicalEntity)
	for _, e := range entries {
		key := strings.TrimSpace(e.original)
		if key == "" {
			continue
		}
		if existing, ok := groups[key]; ok {
			existing.SourceDocIDs = append(existing.SourceDocIDs, e.docID)
			existing.SourceDocIDs = er.uniqueInts(existing.SourceDocIDs)
			existing.Aliases = append(existing.Aliases, e.aliases...)
			existing.Aliases = er.uniqueStrings(existing.Aliases)
			groups[key] = existing
		} else {
			groups[key] = CanonicalEntity{
				Name:         key,
				Type:         "concept",
				SourceDocIDs: []int{e.docID},
				Aliases:      e.aliases,
				Confidence:   "high",
			}
		}
	}
	return groups
}

func (er *EntityResolver) fuzzyMatch(exactGroups map[string]CanonicalEntity) map[string]CanonicalEntity {
	merged := make(map[string]bool)
	fuzzyResults := make(map[string]CanonicalEntity)

	names := make([]string, 0, len(exactGroups))
	for name := range exactGroups {
		names = append(names, name)
	}

	for i := 0; i < len(names); i++ {
		if merged[names[i]] {
			continue
		}
		base := exactGroups[names[i]]
		baseLower := strings.ToLower(names[i])

		for j := i + 1; j < len(names); j++ {
			if merged[names[j]] {
				continue
			}
			candidate := exactGroups[names[j]]
			candidateLower := strings.ToLower(names[j])

			if er.isFuzzyMatch(baseLower, candidateLower, base, candidate) {
				base.Aliases = append(base.Aliases, names[j])
				base.SourceDocIDs = append(base.SourceDocIDs, candidate.SourceDocIDs...)
				base.SourceDocIDs = er.uniqueInts(base.SourceDocIDs)
				base.Aliases = er.uniqueStrings(base.Aliases)
				base.Confidence = "medium"
				merged[names[j]] = true
			}
		}
		fuzzyResults[names[i]] = base
	}

	return fuzzyResults
}

func (er *EntityResolver) isFuzzyMatch(a, b string, aEntity, bEntity CanonicalEntity) bool {
	if a == b {
		return true
	}

	if len(a) > 2 && len(b) > 2 && (strings.Contains(a, b) || strings.Contains(b, a)) {
		return true
	}

	aRunes := []rune(a)
	bRunes := []rune(b)
	if len(aRunes) > 1 && len(bRunes) > 1 {
		common := er.commonPrefixLength(aRunes, bRunes)
		if common >= 2 && (float64(common)/float64(max(len(aRunes), len(bRunes)))) > 0.5 {
			return true
		}
	}

	aliasSet := make(map[string]bool)
	for _, alias := range aEntity.Aliases {
		aliasSet[strings.ToLower(strings.TrimSpace(alias))] = true
	}
	for _, alias := range bEntity.Aliases {
		if aliasSet[strings.ToLower(strings.TrimSpace(alias))] {
			return true
		}
	}

	return false
}

func (er *EntityResolver) commonPrefixLength(a, b []rune) int {
	maxLen := len(a)
	if len(b) < maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		if unicode.ToLower(a[i]) != unicode.ToLower(b[i]) {
			return i
		}
	}
	return maxLen
}

func (er *EntityResolver) llmResolve(ctx context.Context, candidates []CanonicalEntity) []CanonicalEntity {
	if len(candidates) < 2 {
		return candidates
	}

	llm := er.provider
	if llm == nil {
		return candidates
	}

	var ambiguousPairs []string
	seen := make(map[string]bool)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			key := fmt.Sprintf("%s|%s", candidates[i].Name, candidates[j].Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			ambiguousPairs = append(ambiguousPairs, fmt.Sprintf("%s ↔ %s", candidates[i].Name, candidates[j].Name))
		}
	}

	if len(ambiguousPairs) == 0 {
		return candidates
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`以下是一些已提取的实体名称对，请判断每对是否指向同一事物（语义等价）。
只输出JSON数组，true表示同一实体，false表示不同实体：
[{"a":"实体A","b":"实体B","same":true}]

待判断的实体对：
%s`, strings.Join(ambiguousPairs, "\n"))

	resp, err := llm.Chat(ctx, "你是一个知识实体归并专家，只输出JSON数组。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[EntityResolver] LLM resolve error: %v", err)
		return candidates
	}

	type mergePair struct {
		A    string `json:"a"`
		B    string `json:"b"`
		Same bool   `json:"same"`
	}
	var pairs []mergePair
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &pairs); err != nil {
		return candidates
	}

	entityMap := make(map[string]*CanonicalEntity)
	for i := range candidates {
		entityMap[candidates[i].Name] = &candidates[i]
	}

	merged := make(map[string]bool)
	for _, p := range pairs {
		if !p.Same {
			continue
		}
		a, okA := entityMap[p.A]
		b, okB := entityMap[p.B]
		if !okA || !okB || merged[p.A] || merged[p.B] {
			continue
		}
		a.Aliases = append(a.Aliases, b.Name)
		a.Aliases = append(a.Aliases, b.Aliases...)
		a.Aliases = er.uniqueStrings(a.Aliases)
		a.SourceDocIDs = append(a.SourceDocIDs, b.SourceDocIDs...)
		a.SourceDocIDs = er.uniqueInts(a.SourceDocIDs)
		a.Confidence = "high"
		merged[p.B] = true
	}

	var result []CanonicalEntity
	for _, e := range candidates {
		if !merged[e.Name] {
			result = append(result, e)
		}
	}
	return result
}

func (er *EntityResolver) uniqueInts(slice []int) []int {
	seen := make(map[int]bool)
	var result []int
	for _, v := range slice {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

func (er *EntityResolver) uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range slice {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
