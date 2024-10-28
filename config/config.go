// config/config.go
package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config содержит все настройки
type Config struct {
	BotToken string
	AdminID  int64
}

// LoadConfig загружает конфигурацию из .env файла
func LoadConfig() Config {
	// Загружаем переменные окружения из .env файла
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Ошибка загрузки файла .env: %v", err)
	}

	// Читаем токен бота
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatalf("Переменная TELEGRAM_BOT_TOKEN не установлена")
	}

	// Читаем AdminID и конвертируем в int64
	adminIDStr := os.Getenv("ADMIN_ID")
	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Невозможно преобразовать ADMIN_ID в int64: %v", err)
	}

	return Config{
		BotToken: botToken,
		AdminID:  adminID,
	}
}
