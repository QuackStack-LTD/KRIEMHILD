# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY index.html vite.config.js ./
COPY src ./src
COPY tests/detail-camera.test.mjs tests/water-surface.test.mjs tests/autosave.test.mjs tests/world-project.test.mjs ./tests/
RUN node --test tests/*.test.mjs && npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./... && go vet ./...
ARG TARGETOS
ARG TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/kriemhild ./cmd/kriemhild

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates \
    && addgroup -g 10001 kriemhild \
    && adduser -D -H -u 10001 -G kriemhild kriemhild \
    && mkdir -p /app /data \
    && chown 10001:10001 /data
WORKDIR /app
COPY --from=backend --chmod=0555 /out/kriemhild /app/kriemhild
COPY --from=frontend /src/dist /app/dist
ENV KRIEMHILD_ADDR=0.0.0.0:8124 \
    KRIEMHILD_DIST=/app/dist \
    KRIEMHILD_DATA_DIR=/data
USER 10001:10001
EXPOSE 8124
VOLUME ["/data"]
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=20s --timeout=5s --start-period=20s --retries=3 CMD ["/app/kriemhild", "-healthcheck"]
ENTRYPOINT ["/app/kriemhild"]
