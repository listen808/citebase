# NOTICE

Third-party software components used by Citebase.
Citebase itself is MIT-licensed (see [LICENSE](LICENSE)); that covers **our** code only.
The components below keep their own licenses, which are reproduced by name here and shipped
inside each dependency. **This file is generated — do not edit it by hand.**

```bash
node tools/gen-notice.mjs   # rewrites NOTICE.md from the current build list
go test ./tools/repocheck/ -run TestNoticeCoversBuildList   # fails if it drifts
```

Measured against HEAD on 2026-10-19:
102 Go modules actually linked into the binary (not the `219` entries in the
module graph), and the direct npm dependencies of the web app.

## Go dependencies

Scope: modules providing at least one package imported by `./app/server` or `./tools/...`.
License is read from the dependency’s own LICENSE file in the module cache.

### Apache-2.0 — 27

github.com/RoaringBitmap/roaring@v1.2.3, github.com/blevesearch/bleve/v2@v2.3.10, github.com/blevesearch/bleve_index_api@v1.0.6, github.com/blevesearch/geo@v0.1.18, github.com/blevesearch/scorch_segment_api/v2@v2.1.6, github.com/blevesearch/segment@v0.9.1, github.com/blevesearch/upsidedown_store_api@v1.0.2, github.com/blevesearch/vellum@v1.0.10, github.com/blevesearch/zapx/v11@v11.3.10, github.com/blevesearch/zapx/v12@v12.3.10, github.com/blevesearch/zapx/v13@v13.3.10, github.com/blevesearch/zapx/v14@v14.3.10, github.com/blevesearch/zapx/v15@v15.3.13, github.com/coreos/go-oidc/v3@v3.20.0, github.com/go-ego/gse@v1.0.2, github.com/go-jose/go-jose/v4@v4.1.4, github.com/go-openapi/jsonpointer@v0.19.5, github.com/go-openapi/jsonreference@v0.19.6, github.com/go-openapi/spec@v0.20.4, github.com/go-openapi/swag@v0.19.15, github.com/golang/geo@v0.0.0-20210211234256-740aa86cb551, github.com/modern-go/concurrent@v0.0.0-20180306012644-bacd9c7ef1dd, github.com/modern-go/reflect2@v1.0.2, github.com/spf13/afero@v1.11.0, github.com/xwb1989/sqlparser@v0.0.0-20180606152119-120387863bf2, gopkg.in/ini.v1@v1.67.0, gopkg.in/yaml.v2@v2.4.0

### BSD-2-Clause — 4

github.com/gorilla/websocket@v1.5.3, github.com/magiconair/properties@v1.8.7, github.com/redis/go-redis/v9@v9.21.0, github.com/vcaesar/cedar@v0.30.0

### BSD-3-Clause — 22

github.com/PuerkitoBio/purell@v1.1.1, github.com/PuerkitoBio/urlesc@v0.0.0-20170810143723-de5bf2ad4578, github.com/bits-and-blooms/bitset@v1.2.0, github.com/blevesearch/mmap-go@v1.0.4, github.com/blevesearch/snowballstem@v0.9.0, github.com/fsnotify/fsnotify@v1.7.0, github.com/go-json-experiment/json@v0.0.0-20260623181947-01eb4420fa68, github.com/golang/protobuf@v1.5.3, github.com/golang/snappy@v0.0.4, github.com/ledongthuc/pdf@v0.0.0-20250511090121-5959a4027728, github.com/sagikazarmark/slog-shim@v0.1.0, github.com/shirou/gopsutil/v3@v3.24.5, github.com/spf13/pflag@v1.0.5, golang.org/x/crypto@v0.54.0, golang.org/x/mod@v0.37.0, golang.org/x/net@v0.57.0, golang.org/x/oauth2@v0.36.0, golang.org/x/sync@v0.22.0, golang.org/x/sys@v0.47.0, golang.org/x/text@v0.40.0, golang.org/x/tools@v0.47.0, google.golang.org/protobuf@v1.33.0

### MIT — 47

github.com/KyleBanks/depth@v1.2.1, github.com/blevesearch/go-porterstemmer@v1.0.3, github.com/blevesearch/gtreap@v0.1.1, github.com/cespare/xxhash/v2@v2.3.0, github.com/chromedp/cdproto@v0.0.0-20260714215040-dc233986426f, github.com/chromedp/chromedp@v0.16.0, github.com/chromedp/sysutil@v1.1.0, github.com/gabriel-vasile/mimetype@v1.4.2, github.com/gin-contrib/sse@v0.1.0, github.com/gin-gonic/gin@v1.9.1, github.com/go-mail/mail/v2@v2.3.0, github.com/go-ole/go-ole@v1.2.6, github.com/go-playground/locales@v0.14.1, github.com/go-playground/universal-translator@v0.18.1, github.com/go-playground/validator/v10@v10.14.0, github.com/gobwas/httphead@v0.1.0, github.com/gobwas/pool@v0.2.1, github.com/gobwas/ws@v1.4.0, github.com/golang-jwt/jwt/v5@v5.2.0, github.com/jinzhu/inflection@v1.0.0, github.com/jinzhu/now@v1.1.5, github.com/josharian/intern@v1.0.0, github.com/json-iterator/go@v1.1.12, github.com/leodido/go-urn@v1.2.4, github.com/lib/pq@v1.12.3, github.com/mailru/easyjson@v0.7.6, github.com/mattn/go-isatty@v0.0.19, github.com/mitchellh/mapstructure@v1.5.0, github.com/mozillazg/go-pinyin@v0.21.0, github.com/pelletier/go-toml/v2@v2.1.0, github.com/robfig/cron/v3@v3.0.1, github.com/sijms/go-ora/v2@v2.9.0, github.com/spf13/cast@v1.6.0, github.com/spf13/viper@v1.18.2, github.com/subosito/gotenv@v1.6.0, github.com/swaggo/files@v1.0.1, github.com/swaggo/gin-swagger@v1.6.1, github.com/swaggo/swag@v1.16.6, github.com/ugorji/go/codec@v1.2.11, github.com/yuin/goldmark@v1.8.2, github.com/yusufpapurcu/wmi@v1.2.4, go.etcd.io/bbolt@v1.3.7, go.uber.org/atomic@v1.11.0, gopkg.in/natefinch/lumberjack.v2@v2.2.1, gopkg.in/yaml.v3@v3.0.1, gorm.io/driver/mysql@v1.5.2, gorm.io/gorm@v1.25.11

### MPL-2.0 — 2

github.com/go-sql-driver/mysql@v1.7.0, github.com/hashicorp/hcl@v1.0.0

## Note on MPL-2.0

MPL-2.0 is **file-level** copyleft: it can ship inside an MIT project, but any change to
those specific library files must itself be published under MPL-2.0, and the MPL text
must stay with them. Its own §10.2 additionally names GNU Affero GPL v3 and GPL v2 as
compatible licenses for those files — that text is why a keyword scan reports MPL as
AGPL, so treat anything in this repo that says otherwise as a scanner bug.

## Web (npm) direct dependencies

Scope: `dependencies` in `web/package.json`. Transitive npm packages are fetched and
distributed by `npm install` itself, each with its own license file, and are therefore not
declared here. License names come from each package manifest’s `license` field.

### (MPL-2.0 OR Apache-2.0) — 1

dompurify

### Apache-2.0 — 1

echarts

### BSD-3-Clause — 1

highlight.js

### ISC — 1

lucide-vue-next

### MIT — 19

@vue-flow/background, @vue-flow/controls, @vue-flow/core, @vue-flow/minimap, axios, katex, lib0, marked, mermaid, monaco-editor, morphicons, pinia, pinia-plugin-persistedstate, vue, vue-i18n, vue-router, y-protocols, y-websocket, yjs

<!-- machine-readable coverage list; tools/repocheck/notice_test.go compares this block
     against `go list -deps` and web/package.json. Keep it last. -->
```
go	github.com/KyleBanks/depth@v1.2.1
go	github.com/PuerkitoBio/purell@v1.1.1
go	github.com/PuerkitoBio/urlesc@v0.0.0-20170810143723-de5bf2ad4578
go	github.com/RoaringBitmap/roaring@v1.2.3
go	github.com/bits-and-blooms/bitset@v1.2.0
go	github.com/blevesearch/bleve/v2@v2.3.10
go	github.com/blevesearch/bleve_index_api@v1.0.6
go	github.com/blevesearch/geo@v0.1.18
go	github.com/blevesearch/go-porterstemmer@v1.0.3
go	github.com/blevesearch/gtreap@v0.1.1
go	github.com/blevesearch/mmap-go@v1.0.4
go	github.com/blevesearch/scorch_segment_api/v2@v2.1.6
go	github.com/blevesearch/segment@v0.9.1
go	github.com/blevesearch/snowballstem@v0.9.0
go	github.com/blevesearch/upsidedown_store_api@v1.0.2
go	github.com/blevesearch/vellum@v1.0.10
go	github.com/blevesearch/zapx/v11@v11.3.10
go	github.com/blevesearch/zapx/v12@v12.3.10
go	github.com/blevesearch/zapx/v13@v13.3.10
go	github.com/blevesearch/zapx/v14@v14.3.10
go	github.com/blevesearch/zapx/v15@v15.3.13
go	github.com/cespare/xxhash/v2@v2.3.0
go	github.com/chromedp/cdproto@v0.0.0-20260714215040-dc233986426f
go	github.com/chromedp/chromedp@v0.16.0
go	github.com/chromedp/sysutil@v1.1.0
go	github.com/coreos/go-oidc/v3@v3.20.0
go	github.com/fsnotify/fsnotify@v1.7.0
go	github.com/gabriel-vasile/mimetype@v1.4.2
go	github.com/gin-contrib/sse@v0.1.0
go	github.com/gin-gonic/gin@v1.9.1
go	github.com/go-ego/gse@v1.0.2
go	github.com/go-jose/go-jose/v4@v4.1.4
go	github.com/go-json-experiment/json@v0.0.0-20260623181947-01eb4420fa68
go	github.com/go-mail/mail/v2@v2.3.0
go	github.com/go-ole/go-ole@v1.2.6
go	github.com/go-openapi/jsonpointer@v0.19.5
go	github.com/go-openapi/jsonreference@v0.19.6
go	github.com/go-openapi/spec@v0.20.4
go	github.com/go-openapi/swag@v0.19.15
go	github.com/go-playground/locales@v0.14.1
go	github.com/go-playground/universal-translator@v0.18.1
go	github.com/go-playground/validator/v10@v10.14.0
go	github.com/go-sql-driver/mysql@v1.7.0
go	github.com/gobwas/httphead@v0.1.0
go	github.com/gobwas/pool@v0.2.1
go	github.com/gobwas/ws@v1.4.0
go	github.com/golang-jwt/jwt/v5@v5.2.0
go	github.com/golang/geo@v0.0.0-20210211234256-740aa86cb551
go	github.com/golang/protobuf@v1.5.3
go	github.com/golang/snappy@v0.0.4
go	github.com/gorilla/websocket@v1.5.3
go	github.com/hashicorp/hcl@v1.0.0
go	github.com/jinzhu/inflection@v1.0.0
go	github.com/jinzhu/now@v1.1.5
go	github.com/josharian/intern@v1.0.0
go	github.com/json-iterator/go@v1.1.12
go	github.com/ledongthuc/pdf@v0.0.0-20250511090121-5959a4027728
go	github.com/leodido/go-urn@v1.2.4
go	github.com/lib/pq@v1.12.3
go	github.com/magiconair/properties@v1.8.7
go	github.com/mailru/easyjson@v0.7.6
go	github.com/mattn/go-isatty@v0.0.19
go	github.com/mitchellh/mapstructure@v1.5.0
go	github.com/modern-go/concurrent@v0.0.0-20180306012644-bacd9c7ef1dd
go	github.com/modern-go/reflect2@v1.0.2
go	github.com/mozillazg/go-pinyin@v0.21.0
go	github.com/pelletier/go-toml/v2@v2.1.0
go	github.com/redis/go-redis/v9@v9.21.0
go	github.com/robfig/cron/v3@v3.0.1
go	github.com/sagikazarmark/slog-shim@v0.1.0
go	github.com/shirou/gopsutil/v3@v3.24.5
go	github.com/sijms/go-ora/v2@v2.9.0
go	github.com/spf13/afero@v1.11.0
go	github.com/spf13/cast@v1.6.0
go	github.com/spf13/pflag@v1.0.5
go	github.com/spf13/viper@v1.18.2
go	github.com/subosito/gotenv@v1.6.0
go	github.com/swaggo/files@v1.0.1
go	github.com/swaggo/gin-swagger@v1.6.1
go	github.com/swaggo/swag@v1.16.6
go	github.com/ugorji/go/codec@v1.2.11
go	github.com/vcaesar/cedar@v0.30.0
go	github.com/xwb1989/sqlparser@v0.0.0-20180606152119-120387863bf2
go	github.com/yuin/goldmark@v1.8.2
go	github.com/yusufpapurcu/wmi@v1.2.4
go	go.etcd.io/bbolt@v1.3.7
go	go.uber.org/atomic@v1.11.0
go	golang.org/x/crypto@v0.54.0
go	golang.org/x/mod@v0.37.0
go	golang.org/x/net@v0.57.0
go	golang.org/x/oauth2@v0.36.0
go	golang.org/x/sync@v0.22.0
go	golang.org/x/sys@v0.47.0
go	golang.org/x/text@v0.40.0
go	golang.org/x/tools@v0.47.0
go	google.golang.org/protobuf@v1.33.0
go	gopkg.in/ini.v1@v1.67.0
go	gopkg.in/natefinch/lumberjack.v2@v2.2.1
go	gopkg.in/yaml.v2@v2.4.0
go	gopkg.in/yaml.v3@v3.0.1
go	gorm.io/driver/mysql@v1.5.2
go	gorm.io/gorm@v1.25.11
npm	@vue-flow/background
npm	@vue-flow/controls
npm	@vue-flow/core
npm	@vue-flow/minimap
npm	axios
npm	dompurify
npm	echarts
npm	highlight.js
npm	katex
npm	lib0
npm	lucide-vue-next
npm	marked
npm	mermaid
npm	monaco-editor
npm	morphicons
npm	pinia
npm	pinia-plugin-persistedstate
npm	vue
npm	vue-i18n
npm	vue-router
npm	y-protocols
npm	y-websocket
npm	yjs
```
