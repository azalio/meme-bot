# Используем официальный образ Go для сборки
FROM golang:1.23 AS builder

# Устанавливаем корневые сертификаты
RUN apt-get update && apt-get install -y ca-certificates

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы go.mod и go.sum для загрузки зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
# RUN go mod download

# Копируем исходный код в контейнер
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o meme-bot ./cmd/main.go

# Используем минимальный образ для финального контейнера
FROM debian:stable-slim

# Копируем корневые сертификаты
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Копируем собранное приложение из builder
COPY --from=builder /app/meme-bot /usr/local/bin/meme-bot

# Устанавливаем рабочую директорию
WORKDIR /app

# Указываем порт, который будет использоваться
EXPOSE 8081

# Команда для запуска приложения
CMD ["meme-bot"]
