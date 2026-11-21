# ============================================
# Stage 1: Build frontend
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /build/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci --registry=https://registry.npmmirror.com

COPY web/ ./
RUN npm run build

# ============================================
# Stage 2: Build backend
# ============================================
FROM golang:1.26-alpine AS backend-builder

WORKDIR /build

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# 中文分词词典（gse）。代码的候选清单是「exe 同级 dict/zh」「exe 同级 data/dict/zh」「CWD/data/dict/zh」，
# 见 internal/service/search/tokenizer_jieba.go 的 jiebaDictCandidates。
# 落点必须选在**卷挂载点之外**：compose 把命名卷挂在 /app/data，一个空卷会把镜像里的
# /app/data/dict 整个盖掉，词典就又没了 —— 表现不是报错，是中文分词静默退化成"整段一个 token"，
# 搜索召回彻底丢失（当初 Windows 免安装包"装完搜索不可用"就是同一种失效）。
# 所以这里收到 /build/dict/zh，运行阶段落到 /app/dict/zh（在候选清单里，且不在卷底下）。
RUN mkdir -p /build/dict && \
    cp -r "$(go env GOMODCACHE)"/github.com/go-ego/gse@*/data/dict/zh /build/dict/zh

# Copy source and build
COPY . .
COPY --from=frontend-builder /build/web/dist ./web/dist

# P2-#49: 移除硬编码 GOARCH=amd64，由 buildx --platform 自动注入 TARGETARCH
# 使用时：docker buildx build --platform linux/amd64,linux/arm64 -t ... --push .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
# 构建元数据：镜像标签之外还要能在容器里问出"这是哪个 commit 建的"。
# Version 不在这里注入 —— internal/version 里它是 const，-X 注不进去（会静默不生效）。
ARG COMMIT=dev
ARG BUILD_DATE=
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -ldflags="-s -w -X github.com/listen808/citebase/internal/version.Commit=${COMMIT} -X github.com/listen808/citebase/internal/version.BuildDate=${BUILD_DATE}" \
      -o citebase ./app/server

# ============================================
# Stage 3: Runtime
# ============================================
FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && addgroup -S citebase \
    && adduser -S citebase -G citebase

WORKDIR /app

# Copy binary and static assets
COPY --from=backend-builder /build/citebase .
COPY --from=backend-builder /build/web/dist ./web/dist
COPY --from=backend-builder /build/configs/config_sample.yaml ./configs/config_sample.yaml
# 中文分词词典（见 Stage 2 的说明）：放在 /app/dict 而不是 /app/data/dict，
# 因为 /app/data 会被 compose 的命名卷盖掉。
COPY --from=backend-builder /build/dict/zh ./dict/zh
ENV AIWIKI_JIEBA_DICT=/app/dict/zh

# Create data directories
RUN mkdir -p data/logs data/images data/attachment data/vectors data/indexes data/markdowns \
    && chown -R citebase:citebase /app

USER citebase

# Expose backend port
EXPOSE 3001

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:3001/api/v1/health || exit 1

# Run the binary
CMD ["./citebase"]
