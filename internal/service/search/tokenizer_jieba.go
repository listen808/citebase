package search

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2/analysis"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	_ "github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	_ "github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/registry"
	"github.com/go-ego/gse"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/utils/envname"
)

const JiebaTokenizerName = "jieba_tokenizer" //nolint:gosec // G101: tokenizer name, not a credential

// jiebaDictRetryInterval 词典加载失败后的重试间隔。
// 失败时不能每次分词都去读盘并刷一条 WARN（部署错词典时会被搜索请求打成日志洪水）。
const jiebaDictRetryInterval = 5 * time.Minute

var (
	jiebaSeg      *gse.Segmenter
	jiebaMu       sync.Mutex
	jiebaInited   bool
	jiebaNextTry  time.Time
	jiebaWarnOnce sync.Once
)

// jiebaDictCandidates 返回词典的候选位置（按优先级）。
//
// gse 的无参 LoadDict() 会把词典目录算成「编译期 gse 包源码目录 + /data」，也就是**构建机的
// Go module cache 绝对路径**（例如 C:/Users/<builduser>/go/pkg/mod/github.com/go-ego/gse@v1.0.2/...）。
// 一旦换机器部署，这个路径必然不存在，中文分词静默退化成「整段一个 token」，
// 搜索召回彻底丢失 —— 这正是 scripts/package-windows.bat 打包出去后搜索不可用的根因。
func jiebaDictCandidates() []string {
	var dirs []string
	if env := strings.TrimSpace(envname.Getenv("CITEBASE_JIEBA_DICT", "AIWIKI_JIEBA_DICT")); env != "" {
		dirs = append(dirs, env)
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		dirs = append(dirs,
			filepath.Join(base, "data", "dict", "zh"),
			filepath.Join(base, "dict", "zh"),
		)
	}
	dirs = append(dirs,
		filepath.Join("data", "dict", "zh"), // 相对工作目录（start.bat 会 cd 到程序目录）
	)
	return dirs
}

// resolveJiebaDict 找出实际存在的词典文件。返回逗号拼接的路径（gse.LoadDict 接受显式文件路径），
// 找不到时返回空串，由调用方回退到 gse 默认逻辑。
func resolveJiebaDict() string {
	for _, dir := range jiebaDictCandidates() {
		info, err := os.Stat(dir)
		if err != nil {
			continue
		}
		var files []string
		if info.IsDir() {
			for _, name := range []string{"s_1.txt", "t_1.txt"} {
				p := filepath.Join(dir, name)
				if _, err := os.Stat(p); err == nil {
					files = append(files, p)
				}
			}
		} else {
			// 环境变量直接指向文件
			files = append(files, dir)
		}
		if len(files) > 0 {
			return strings.Join(files, ",")
		}
	}
	return ""
}

// initJieba 加载词典；失败时不置位 inited，超过重试间隔后会再次尝试。
// 旧实现用 sync.Once 包住可失败加载：启动瞬间词典缺失/IO 失败后 jiebaInited 仍被置 true，
// 进程内永不重试，全部中文文本静默退化为 fallback 单 token，搜索召回彻底丢失。
func initJieba() {
	jiebaMu.Lock()
	defer jiebaMu.Unlock()
	if jiebaInited {
		return
	}
	if time.Now().Before(jiebaNextTry) {
		return
	}
	jiebaNextTry = time.Now().Add(jiebaDictRetryInterval)

	var seg gse.Segmenter
	// AlphaNum：让连续字母/数字作为一个词进入分词，而不是被逐字符切碎或直接丢弃。
	// 不开启时 gse 对纯 ASCII 文本（如 "A"、"Grafana"）常常整段不产出 token，
	// 表现为「搜英文/字母关键词永远 0 结果」。
	seg.AlphaNum = true
	dict := resolveJiebaDict()
	var err error
	if dict != "" {
		err = seg.LoadDict(dict)
	} else {
		// 找不到随包词典时仍尝试 gse 默认路径（开发机源码目录场景）
		err = seg.LoadDict()
	}
	if err != nil {
		jiebaWarnOnce.Do(func() {
			logx.Errorf("[Search] jieba 词典加载失败，中文搜索将降级为 CJK 二元组分词。"+
				"请把词典放到 <程序目录>\\data\\dict\\zh\\s_1.txt（打包脚本 scripts/package-windows.bat 会自动复制），"+
				"或用 CITEBASE_JIEBA_DICT 指定词典目录/文件（改名前的旧名仍可读）。原因: %v", err)
		})
		return
	}
	jiebaSeg = &seg
	jiebaInited = true
	source := dict
	if source == "" {
		source = "gse 默认路径"
	}
	logx.Infof("[Search] jieba 词典加载完成: %s", source)
}

type JiebaTokenizer struct{}

func NewJiebaTokenizer() *JiebaTokenizer {
	initJieba()
	return &JiebaTokenizer{}
}

func (t *JiebaTokenizer) Tokenize(input []byte) analysis.TokenStream {
	jiebaMu.Lock()
	seg, ok := jiebaSeg, jiebaInited
	jiebaMu.Unlock()
	if !ok || seg == nil {
		return fallbackTokenize(input)
	}

	// 按「ASCII 词 / CJK 串」分段处理：
	//  1) gse 对纯 ASCII 文本经常整段不产出 token（历史上搜 "A"、"Grafana" 恒为 0 结果），
	//     开启 AlphaNum 后又会把 "AB" 切成 a、b 两个单字母，导致 "AB" 命中 "B模块" 这类噪声；
	//     所以字母数字串由我们自己切成完整词，只把中文串交给 gse 做词典分词。
	//  2) CJK 整词额外补「单字」子 token（共享父 token 的 position，不打断短语连续性），
	//     否则词典把「集团」当整词入库后，用户搜「集」这种局部词永远命中不了。
	//  3) token 的 Start/End 始终是原文字节偏移（旧实现用 CutSearch 的字符串列表累加推算，
	//     搜索引擎模式的重叠子词会让偏移漂移，短语与高亮定位失准）。
	result := make(analysis.TokenStream, 0, 16)
	emittedSub := make(map[string]bool, 16)
	position := 0

	appendToken := func(term []byte, start, end int, pos int) {
		if len(term) == 0 || end <= start {
			return
		}
		result = append(result, &analysis.Token{Term: term, Start: start, End: end, Position: pos})
	}

	i := 0
	for i < len(input) {
		r, size := utf8.DecodeRune(input[i:])
		switch {
		case isASCIILetterDigit(r):
			start := i
			for i < len(input) {
				r2, sz := utf8.DecodeRune(input[i:])
				if !isASCIILetterDigit(r2) {
					break
				}
				i += sz
			}
			position++
			appendToken(input[start:i], start, i, position)
			// 字母数字混合串（如 Grafana2024）额外拆出纯字母段与数字段，
			// 让「Grafana」也能命中「Grafana2024」。
			if hasLetterBytes(input[start:i]) && hasDigit(input[start:i]) {
				emitMixedParts(input, start, i, &position, appendToken)
			}

		case isCJKRune(r):
			start := i
			for i < len(input) {
				r2, sz := utf8.DecodeRune(input[i:])
				if !isCJKRune(r2) {
					break
				}
				i += sz
			}
			// 传独立副本而不是原数组的子切片：子切片的 cap 会延伸到整个原文缓冲区，
			// gse 内部按缓冲区长度处理时会把后面的内容一起看进来，导致词项与偏移错乱。
			run := append([]byte(nil), input[start:i]...)
			segments := seg.ModeSegment(run, true)
			for s := range segments {
				text := segments[s].Token().Text()
				if len(text) == 0 {
					continue
				}
				tStart := start + segments[s].Start()
				tEnd := start + segments[s].End()
				if tEnd <= tStart {
					tEnd = tStart + len(text)
				}
				position++
				appendToken([]byte(text), tStart, tEnd, position)
				runes := []rune(text)
				if len(runes) < 2 {
					continue
				}
				off := tStart
				for _, rr := range runes {
					n := utf8.RuneLen(rr)
					key := string(rr)
					if !emittedSub[key] {
						emittedSub[key] = true
						appendToken([]byte(key), off, off+n, position)
					}
					off += n
				}
			}

		default:
			i += size
		}
	}

	if len(result) == 0 {
		// 兜底：保证任何输入都有可匹配词项（标点、全角符号、词典外生僻字符等）
		return fallbackTokenize(input)
	}
	return result
}

// hasDigit / hasLetterBytes 判断 ASCII 串是否同时含字母与数字（混合串需要额外拆词）
func hasDigit(b []byte) bool {
	for _, c := range b {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

func hasLetterBytes(b []byte) bool {
	for _, c := range b {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

// emitMixedParts 为字母数字混合串补充「纯字母段」和「数字段」两个子 token
func emitMixedParts(input []byte, start, end int, position *int, appendToken func([]byte, int, int, int)) {
	j := start
	for j < end {
		k := j
		isDigitPos := input[j] >= '0' && input[j] <= '9'
		for k < end && ((input[k] >= '0' && input[k] <= '9') == isDigitPos) {
			k++
		}
		if k-j > 1 { // 单字符片段噪声太大，跳过
			*position++
			appendToken(input[j:k], j, k, *position)
		}
		j = k
	}
}

func isASCIILetterDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isCJKRune(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Scripts["Hiragana"], r) ||
		unicode.Is(unicode.Scripts["Katakana"], r) ||
		unicode.Is(unicode.Scripts["Hangul"], r)
}

// isCJKText 判断文本是否包含中日韩文字
func isCJKText(s string) bool {
	for _, r := range s {
		if isCJKRune(r) {
			return true
		}
	}
	return false
}

// fallbackTokenize 词典不可用时的降级分词：CJK 单字 + 相邻二元组，字母数字按词切分。
//
// 旧实现把整段文本当成一个 token：任何查询词都命中不了，等于「词典缺失 → 中文搜索彻底不可用」。
// 降级仍要保证可用（与 bleve 的 cjk 分析器同思路，额外补单字以支持单字查询），
// 只是召回精度不如真正的分词。
func fallbackTokenize(input []byte) analysis.TokenStream {
	result := make(analysis.TokenStream, 0, 16)
	position := 0

	// emit 记录一个 token，偏移直接用原文字节位置，保证与 ModeSegment 语义一致
	emit := func(start, end int) {
		if end <= start {
			return
		}
		result = append(result, &analysis.Token{
			Term:     input[start:end],
			Start:    start,
			End:      end,
			Position: position + 1,
		})
		position++
	}

	runes := []rune(string(input))
	// runeOffsets[i] 是第 i 个 rune 的起始字节偏移，末位放总长度便于取 end
	runeOffsets := make([]int, len(runes)+1)
	acc := 0
	for i, r := range runes {
		runeOffsets[i] = acc
		acc += utf8.RuneLen(r)
	}
	runeOffsets[len(runes)] = acc

	isCJK := func(r rune) bool {
		return unicode.Is(unicode.Han, r) ||
			unicode.Is(unicode.Scripts["Hiragana"], r) ||
			unicode.Is(unicode.Scripts["Katakana"], r) ||
			unicode.Is(unicode.Scripts["Hangul"], r)
	}
	isWordChar := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}

	i := 0
	for i < len(runes) {
		switch {
		case isCJK(runes[i]):
			start := i
			for i < len(runes) && isCJK(runes[i]) {
				i++
			}
			for j := start; j < i; j++ {
				emit(runeOffsets[j], runeOffsets[j+1]) // 单字
				if j+1 < i {
					emit(runeOffsets[j], runeOffsets[j+2]) // 二元组
				}
			}
		case isWordChar(runes[i]):
			start := i
			for i < len(runes) && isWordChar(runes[i]) {
				i++
			}
			emit(runeOffsets[start], runeOffsets[i])
		default:
			i++
		}
	}

	if len(result) == 0 {
		return analysis.TokenStream{&analysis.Token{Term: input, Start: 0, End: len(input), Position: 1}}
	}
	return result
}

func JiebaTokenizerConstructor(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
	return NewJiebaTokenizer(), nil
}

func init() {
	registry.RegisterTokenizer(JiebaTokenizerName, JiebaTokenizerConstructor)
}
