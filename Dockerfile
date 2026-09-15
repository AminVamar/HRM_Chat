# ---- сборка ----
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- запуск ----
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/server /app/server
COPY migrations /app/migrations
RUN mkdir -p /app/uploads && chown -R app:app /app

USER app
ENV PORT=8080 UPLOAD_DIR=/app/uploads
EXPOSE 8080
VOLUME ["/app/uploads"]

ENTRYPOINT ["/app/server"]
