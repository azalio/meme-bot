# Meme Bot

Телеграм бот для генерации мемов с использованием Yandex GPT и Yandex Art API.

## Описание

Бот использует Yandex GPT для генерации описания мема на основе пользовательского ввода и Yandex Art для создания изображения.

## Требования

- Go 1.23.2 или выше
- Telegram Bot Token
- Yandex OAuth Token
- Yandex Cloud Folder ID

## Установка

1. Клонировать репозиторий:
```bash
git clone https://github.com/azalio/meme-bot.git
cd meme-bot
```

2. Создать файл .env со следующим содержимым:
```env
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
YANDEX_OAUTH_TOKEN=your_yandex_oauth_token
YANDEX_ART_FOLDER_ID=your_folder_id
```

3. Установить зависимости:
```bash
go mod download
```

4. Собрать проект:
```bash
go build -o meme-bot cmd/main.go
```

## Использование

1. Запустить бота:
```bash
./meme-bot
```

2. В Telegram использовать следующие команды:
- `/start` - Начать работу с ботом
- `/help` - Показать справку
- `/meme [текст]` - Сгенерировать мем с описанием

## Структура проекта

```
.
├── cmd/
│   └── main.go           # Точка входа в приложение
├── internal/
│   ├── config/          # Конфигурация приложения
│   ├── middleware/      # Middleware для логирования и обработки ошибок
│   └── service/         # Бизнес-логика и сервисы
├── pkg/
│   └── logger/          # Пакет для логирования
├── .env                  # Конфигурационные переменные
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

## Архитектурные решения

1. **Clean Architecture** - проект следует принципам чистой архитектуры
2. **Dependency Injection** - используется для управления зависимостями
3. **Interface Segregation** - интерфейсы разделены по принципу единой ответственности
4. **Middleware Pattern** - для обработки запросов и логирования

## Вклад в разработку

Приветствуются любые предложения по улучшению проекта через Issues и Pull Requests.

## Лицензия

MIT