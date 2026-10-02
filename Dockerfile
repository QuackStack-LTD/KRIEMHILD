FROM node:24-alpine AS frontend
WORKDIR /build/apps/web
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci --no-fund
COPY apps/web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -o /kriemhild ./cmd/kriemhild
RUN CGO_ENABLED=0 go build -trimpath -o /kriemhild-tool ./cmd/kriemhild-tool
RUN CGO_ENABLED=0 go build -trimpath -o /kriemhild-admin ./cmd/kriemhild-admin

FROM alpine:3.22
RUN apk add --no-cache ca-certificates git && addgroup -g 10001 app && adduser -D -u 10001 -G app app
WORKDIR /app
COPY --from=backend /kriemhild /kriemhild-tool /kriemhild-admin ./
COPY --from=frontend /build/apps/web/out-dev ./web
COPY LICENSE docs/DEPENDENCY_NOTICES.md ./
RUN mkdir /data && chown app:app /data
USER app
VOLUME /data
EXPOSE 4784
ENTRYPOINT ["/app/kriemhild", "-data", "/data", "-web", "/app/web"]
# For hosted access pass -addr 0.0.0.0:4784 -origin https://your-host.
# The initial admin password is read from KRIEMHILD_ADMIN_PASSWORD.
