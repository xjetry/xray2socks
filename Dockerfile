FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/x2socks .

FROM alpine:3.21 AS xray
RUN apk add --no-cache curl unzip ca-certificates
ARG TARGETARCH
ARG GH_PROXY=
RUN case "$TARGETARCH" in \
      amd64) zip=Xray-linux-64.zip ;; \
      arm64) zip=Xray-linux-arm64-v8a.zip ;; \
      *) echo "unsupported arch: $TARGETARCH" >&2; exit 1 ;; \
    esac \
 && curl -fsSL "${GH_PROXY}https://github.com/XTLS/Xray-core/releases/latest/download/$zip" -o /tmp/xray.zip \
 && unzip -o -j -q /tmp/xray.zip xray -d /usr/local/bin \
 && chmod +x /usr/local/bin/xray

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /out/x2socks /usr/local/bin/x2socks
COPY --from=xray /usr/local/bin/xray /usr/local/bin/xray
ENV XRAY2SOCKS_CONFIG=/data/config.json \
    XRAY2SOCKS_BIND=0.0.0.0 \
    XRAY2SOCKS_ADDR=0.0.0.0:8080
WORKDIR /data
VOLUME /data
EXPOSE 8080 1080-1090
CMD ["x2socks", "serve"]
