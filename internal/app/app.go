package app

import (
	"log"
	"sync"

	"github.com/VadimBorzenkov/TradeSimulatorBot/config"
	"github.com/VadimBorzenkov/TradeSimulatorBot/internal/bot"
	"github.com/VadimBorzenkov/TradeSimulatorBot/internal/trader"
)

// RunApp запускает все компоненты приложения
func RunApp() {
	cfg := config.LoadConfig()

	// Создаем экземпляр trader.Trader с начальным капиталом 100 долларов
	traderInstance := &trader.Trader{
		Capital:     100.0,
		Investments: []trader.Investment{}, // Начинаем с пустым списком инвестиций
	}

	// Передаем traderInstance в NewTelegramBot
	tgBot := bot.NewTelegramBot(cfg.BotToken, cfg.AdminID, traderInstance)

	// Используем wait group, чтобы программа не завершалась
	var wg sync.WaitGroup
	wg.Add(1)

	// Запуск бота в отдельной горутине
	go func() {
		tgBot.Start()
		wg.Done()
	}()

	log.Println("Приложение запущено")

	wg.Wait()
}
