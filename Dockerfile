# Используем официальный образ Go для сборки
FROM golang:1.23.2-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы go.mod и go.sum для загрузки зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код в контейнер
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o meme-bot cmd/main.go

# Используем пустой образ для финального контейнера
FROM scratch
# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем собранное приложение из builder
COPY --from=builder /app/meme-bot .

# Копируем .env файл (если он используется)
COPY .env .

# Указываем порт, который будет использоваться
EXPOSE 8081

# Команда для запуска приложения
ENTRYPOINT ["./meme-bot"]
