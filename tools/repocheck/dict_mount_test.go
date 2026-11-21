package repocheck

import (
	"regexp"
	"strings"
	"testing"
)

// 这台管一件只有把三份部署文件对着读才看得见的事：**中文分词词典落在容器里的哪个路径，
// 以及那个路径会不会被卷挂载盖掉。**
//
// 起因：gse 的无参 LoadDict() 把词典目录算成"编译期 gse 包源码目录"，也就是构建机的
// module cache 绝对路径；换机器部署时那个路径必然不存在，于是中文分词**静默退化成整段一个 token**
// —— 不报错、不崩、搜索召回彻底丢失。代码为此改了候选清单（exe 同级 dict/zh 等），
// 但 Dockerfile 与 compose 是另外两个文件：compose 把命名卷挂在 /app/data，
// 一个空卷会把镜像里 /app/data/dict 整个盖掉，词典又没了，而同样的静默退化照旧发生。
// 这类"两个文件各自都对、合起来失效"的形态，单文件扫描与运行时巡检都看不见。

var (
	dockerfileDictCopy = regexp.MustCompile(`(?m)^COPY\s+--from=[^\s]+\s+\S*dict/zh\s+\./(\S+)`)
	composeVolume      = regexp.MustCompile(`(?m)^\s*-\s*\w[\w.-]*:(/[^\s]+)`)
	envDict            = regexp.MustCompile(`(?m)^ENV\s+AIWIKI_JIEBA_DICT=(\S+)`)
	codeCandidate      = regexp.MustCompile(`dict",\s*"zh"`)
)

func TestDictIsNotUnderAVolumeMount(t *testing.T) {
	dockerfile := readRepoFile(t, "Dockerfile")
	compose := readRepoFile(t, "docker-compose.yml")
	tokenizer := readRepoFile(t, "internal/service/search/tokenizer_jieba.go")

	mounts := map[string]bool{}
	for _, m := range composeVolume.FindAllStringSubmatch(compose, -1) {
		mounts[strings.TrimSuffix(m[1], "/")] = true
	}
	if len(mounts) == 0 {
		t.Fatal("没从 docker-compose.yml 认出任何卷挂载 —— 判据的范围算错了，此时下面的检查全是空转")
	}

	// 词典在容器里的两个可能落点：ENV 指的路径，以及 COPY 的目标（WORKDIR /app 之下）。
	var dictPaths []string
	for _, m := range envDict.FindAllStringSubmatch(dockerfile, -1) {
		dictPaths = append(dictPaths, m[1])
	}
	for _, m := range dockerfileDictCopy.FindAllStringSubmatch(dockerfile, -1) {
		dictPaths = append(dictPaths, "/app/"+strings.TrimSuffix(m[1], "/"))
	}
	if len(dictPaths) == 0 {
		t.Fatal("Dockerfile 里既没有 AIWIKI_JIEBA_DICT 也没有 dict/zh 的 COPY —— 镜像内的中文词典不见了，" +
			"部署出去的容器会静默失去中文分词")
	}

	for _, p := range dictPaths {
		for mount := range mounts {
			if p == mount || strings.HasPrefix(p, mount+"/") {
				t.Errorf("词典落在 %s，而 %s 是卷挂载点 —— 命名卷一挂上去就把镜像里的词典盖成空目录，"+
					"中文分词静默退化。把词典移到挂载点之外（当前用的是 /app/dict/zh）。", p, mount)
			}
		}
	}

	// 落点必须在代码的候选清单里，否则镜像放对了地方而应用找不到。
	if !codeCandidate.MatchString(tokenizer) {
		t.Error("tokenizer_jieba.go 的候选清单里认不出 dict/zh 这一档 —— Dockerfile 的落点与代码的查找路径已经脱节")
	}
}

// TestDictCopySourceUsesGlob 拦"把 gse 版本写死在拷贝路径里"。
// 升级 gse 的那次提交不会有人想起 Dockerfile —— 而它一旦失配，构建时才报错（甚至 cp 静默空目录），
// 又是同一类"要到部署后才发现"。
func TestDictCopySourceUsesGlob(t *testing.T) {
	dockerfile := readRepoFile(t, "Dockerfile")
	i := strings.Index(dockerfile, "go-ego/gse@")
	if i < 0 {
		t.Fatal("Dockerfile 里没有拷贝 gse 词典的步骤")
	}
	seg := dockerfile[max(0, i-60):min(len(dockerfile), i+60)]
	if strings.Contains(seg, "gse@v") && !strings.Contains(seg, "gse@*") {
		t.Errorf("词典拷贝的源路径钉死了 gse 版本（%q）—— 用 gse@* 让它跟着模块目录走", strings.TrimSpace(seg))
	}
}
