# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o meme-bot cmd/main.go

# Final stage
FROM alpine:3.18
RUN adduser -D -u 1000 appuser
USER appuser
COPY --from=builder /app/meme-bot /meme-bot
EXPOSE 8080
ENTRYPOINT ["/meme-bot"]