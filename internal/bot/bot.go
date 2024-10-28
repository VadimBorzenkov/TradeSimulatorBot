// internal/bot/bot.go
package bot

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/VadimBorzenkov/TradeSimulatorBot/internal/trader"
	"github.com/VadimBorzenkov/TradeSimulatorBot/okx"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramBot содержит структуру для работы с ботом
type TelegramBot struct {
	Bot                 *tgbotapi.BotAPI
	AdminID             int64
	Trader              *trader.Trader
	AwaitingAssetInput  map[int64]bool   // Ожидание ввода актива
	AwaitingBuyInput    map[int64]bool   // Ожидание ввода для покупки
	AwaitingSellInput   map[int64]bool   // Ожидание ввода для продажи
	AwaitingAmountInput map[int64]string // Хранение актива для ввода суммы (buy/sell)
}

func NewTelegramBot(token string, adminID int64, t *trader.Trader) *TelegramBot {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	return &TelegramBot{
		Bot:                 bot,
		AdminID:             adminID,
		Trader:              t, // Сохраняем Trader
		AwaitingAssetInput:  make(map[int64]bool),
		AwaitingBuyInput:    make(map[int64]bool),
		AwaitingSellInput:   make(map[int64]bool),
		AwaitingAmountInput: make(map[int64]string),
	}
}

// Создаем клавиатуру с кнопками
func createReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/trade"),
			tgbotapi.NewKeyboardButton("/assets"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/price"),
		),
	)
}

func createTradeKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/balance"),
			tgbotapi.NewKeyboardButton("/buy"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/sell"),
			tgbotapi.NewKeyboardButton("/grid_strategy"),
		),
	)
}

func (tb *TelegramBot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := tb.Bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			// Обработка команды /start
			if update.Message.Text == "/start" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Бот запущен! Выберите команду.")
				msg.ReplyMarkup = createReplyKeyboard() // Добавляем клавиатуру
				tb.Bot.Send(msg)
				continue
			}

			// Обработка команды /assets
			if update.Message.Text == "/assets" {
				assets, err := okx.GetAssets()
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения активов"))
					continue
				}

				assetList := ""
				for _, asset := range assets {
					assetList += asset + "\n"
				}
				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Список активов:\n"+assetList))
				continue
			}

			// Обработка команды /trade
			if update.Message.Text == "/trade" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Выберите действие:")
				msg.ReplyMarkup = createTradeKeyboard() // Добавляем клавиатуру с кнопками торговли
				tb.Bot.Send(msg)
				continue
			}

			// Обработка команды /price
			if update.Message.Text == "/price" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Введите символ актива (например, BTC-USDT):")
				tb.Bot.Send(msg)
				tb.AwaitingAssetInput[update.Message.Chat.ID] = true // Устанавливаем состояние ожидания ввода актива
				continue
			}

			// Проверяем, является ли введенный текст символом актива
			if tb.AwaitingAssetInput[update.Message.Chat.ID] {
				asset := update.Message.Text
				if !isValidAsset(asset) {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Недействительный актив. Попробуйте снова."))
					continue
				}

				price, err := tb.getPriceWithRetries(asset)
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения цены: "+err.Error()))
					delete(tb.AwaitingAssetInput, update.Message.Chat.ID)
					continue
				}

				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Текущая цена для "+asset+": "+price+"$"))

				// Сбрасываем состояние ожидания актива
				delete(tb.AwaitingAssetInput, update.Message.Chat.ID)
				continue
			}

			// Обработка команды /balance
			if update.Message.Text == "/balance" {
				balance, err := tb.Trader.GetBalance()
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения баланса: "+err.Error()))
					continue
				}

				// Начинаем формировать сообщение о балансе
				balanceMessage := "Текущие активы:\n"

				// Добавляем токен USDT с текущим балансом
				usdtValue := tb.Trader.Capital
				balanceMessage += fmt.Sprintf("Токен: USDT, Количество: %.2f, Общая стоимость: $%.2f\n", usdtValue, usdtValue)

				// Перебираем все инвестиции и добавляем их в сообщение
				for _, investment := range balance.Investments {
					currentPrice, err := okx.GetCurrentPrice(investment.Token)
					if err != nil {
						balanceMessage += fmt.Sprintf("Ошибка получения цены для %s\n", investment.Token)
						continue
					}

					price, _ := strconv.ParseFloat(currentPrice, 64)
					totalValue := investment.Amount * price
					balanceMessage += fmt.Sprintf("Токен: %s, Количество: %.2f, Общая стоимость: $%.2f\n",
						investment.Token, investment.Amount, totalValue)
				}

				// Суммируем общую стоимость всех активов
				totalAssetsValue := usdtValue
				for _, investment := range balance.Investments {
					currentPrice, err := okx.GetCurrentPrice(investment.Token)
					if err == nil {
						price, _ := strconv.ParseFloat(currentPrice, 64)
						totalAssetsValue += investment.Amount * price
					}
				}

				balanceMessage += fmt.Sprintf("Общая стоимость: $%.2f", totalAssetsValue)

				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, balanceMessage))
				continue
			}

			// Обработка команды /buy
			if update.Message.Text == "/buy" {
				usdtValue := tb.Trader.GetCapital()
				tokenList := "Доступные токены для покупки:\n"

				// Получаем список доступных токенов
				availableTokens := []string{"BTC-USDT", "ETH-USDT", "LTC-USDT", "XRP-USDT", "ADA-USDT", "SOL-USDT", "DOT-USDT", "BNB-USDT", "DOGE-USDT", "LINK-USDT"}
				for _, token := range availableTokens {
					tokenList += token + "\n"
				}

				msg := fmt.Sprintf("Ваш баланс USDT: %.2f\n%s", usdtValue, tokenList)
				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))

				tb.AwaitingAssetInput[update.Message.Chat.ID] = true
				tb.AwaitingBuyInput[update.Message.Chat.ID] = true
				continue
			}

			// Проверка, ожидается ли ввод токена для покупки
			if tb.AwaitingAssetInput[update.Message.Chat.ID] && tb.AwaitingBuyInput[update.Message.Chat.ID] {
				asset := update.Message.Text
				if !isValidAsset(asset) {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Недействительный актив. Попробуйте снова."))
					continue
				}

				// Сохраняем актив и просим ввести сумму
				tb.AwaitingAmountInput[update.Message.Chat.ID] = asset
				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Введите сумму для покупки:"))
				delete(tb.AwaitingAssetInput, update.Message.Chat.ID)
				continue
			}

			// Проверка, ожидается ли ввод суммы для покупки
			if asset, awaiting := tb.AwaitingAmountInput[update.Message.Chat.ID]; awaiting {
				amount, err := strconv.ParseFloat(update.Message.Text, 64)
				if err != nil || amount <= 0 {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Недопустимая сумма. Попробуйте снова."))
					continue
				}

				currentPrice, err := okx.GetCurrentPrice(asset) // Получаем текущую цену токена
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения цены: "+err.Error()))
					continue
				}

				price, _ := strconv.ParseFloat(currentPrice, 64)
				totalCost := price * amount

				if totalCost > tb.Trader.GetCapital() {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Недостаточно средств для покупки."))
					continue
				}

				// Выполняем покупку
				err = tb.Trader.BuyToken(asset, amount, price)
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при покупке: "+err.Error()))
					continue
				}

				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Вы успешно купили %.2f токена(ов) %s по цене %.2f$", amount, asset, totalCost)))
				delete(tb.AwaitingAmountInput, update.Message.Chat.ID) // Сбрасываем состояние ожидания суммы
			}

			// Обработка команды /sell
			if update.Message.Text == "/sell" {
				balance, err := tb.Trader.GetBalance()
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения баланса: "+err.Error()))
					continue
				}

				// Формируем сообщение со списком текущих активов
				sellMessage := "Текущие токены для продажи:\n"
				for _, investment := range balance.Investments {
					currentPrice, err := okx.GetCurrentPrice(investment.Token)
					if err != nil {
						sellMessage += fmt.Sprintf("Ошибка получения цены для %s\n", investment.Token)
						continue
					}

					price, _ := strconv.ParseFloat(currentPrice, 64)
					sellMessage += fmt.Sprintf("Токен: %s, Количество: %.2f, Текущая цена: $%.2f\n",
						investment.Token, investment.Amount, price)
				}

				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, sellMessage))
				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Введите символ токена для продажи (например, BTC-USDT):"))
				tb.AwaitingAssetInput[update.Message.Chat.ID] = true
				tb.AwaitingSellInput[update.Message.Chat.ID] = true
				continue
			}

			// Проверка, ожидается ли ввод токена для продажи
			if tb.AwaitingAssetInput[update.Message.Chat.ID] && tb.AwaitingSellInput[update.Message.Chat.ID] {
				asset := update.Message.Text
				if !isValidAsset(asset) {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Недействительный актив. Попробуйте снова."))
					continue
				}

				// Сохраняем актив и просим ввести сумму
				tb.AwaitingAmountInput[update.Message.Chat.ID] = asset
				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Введите количество для продажи (в USDT):"))
				delete(tb.AwaitingAssetInput, update.Message.Chat.ID) // Сбрасываем состояние ожидания актива
				continue
			}

			// Проверка, ожидается ли ввод суммы для продажи
			if asset, awaiting := tb.AwaitingAmountInput[update.Message.Chat.ID]; awaiting {
				amount, err := strconv.ParseFloat(update.Message.Text, 64)
				if err != nil || amount <= 0 {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверная сумма. Попробуйте снова."))
					continue
				}

				currentBalance := 0.0
				for _, investment := range tb.Trader.Investments {
					if investment.Token == asset {
						currentBalance = investment.Amount
						break
					}
				}

				if currentBalance == 0 {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "У вас нет токенов для продажи."))
					delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
					delete(tb.AwaitingSellInput, update.Message.Chat.ID)
					continue
				}

				// Выполняем продажу токена
				currentPriceStr, err := tb.getPriceWithRetries(asset)
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения цены: "+err.Error()))
					delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
					continue
				}

				currentPrice, _ := strconv.ParseFloat(currentPriceStr, 64)
				_, err = tb.Trader.SellToken(asset, amount/currentPrice, currentPrice)
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка продажи токена: "+err.Error()))
					continue
				}

				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Токен "+asset+" успешно продан."))
				delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
				delete(tb.AwaitingSellInput, update.Message.Chat.ID)
			}

			// Обработка команды /grid_strategy
			if update.Message.Text == "/grid_strategy" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Введите символ актива для сеточной стратегии (например, BTC-USDT):")
				tb.Bot.Send(msg)
				tb.AwaitingAssetInput[update.Message.Chat.ID] = true
				// Логика для сеточной стратегии будет добавлена здесь позже
				continue
			}

			// Проверяем, является ли введенный текст символом актива
			if tb.AwaitingAssetInput[update.Message.Chat.ID] {
				asset := update.Message.Text
				if !isValidAsset(asset) {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Недействительный актив. Попробуйте снова."))
					continue
				}

				price, err := tb.getPriceWithRetries(asset)
				if err != nil {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения цены: "+err.Error()))
					delete(tb.AwaitingAssetInput, update.Message.Chat.ID)
					continue
				}

				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Текущая цена для "+asset+": "+price))

				// Сохраняем актив и просим ввести сумму
				tb.AwaitingAmountInput[update.Message.Chat.ID] = asset
				tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Введите сумму для покупки/продажи:"))
				delete(tb.AwaitingAssetInput, update.Message.Chat.ID)
				continue
			}

			// Проверка, ожидается ли ввод суммы для покупки или продажи
			if asset, awaiting := tb.AwaitingAmountInput[update.Message.Chat.ID]; awaiting {
				amount, err := strconv.ParseFloat(update.Message.Text, 64)
				if err != nil || amount <= 0 {
					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверная сумма. Попробуйте снова."))
					continue
				}

				// Проверяем, было ли это покупкой
				if tb.AwaitingBuyInput[update.Message.Chat.ID] {
					priceStr, err := tb.getPriceWithRetries(asset)
					if err != nil {
						tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения цены: "+err.Error()))
						delete(tb.AwaitingBuyInput, update.Message.Chat.ID)
						delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
						continue
					}

					price, _ := strconv.ParseFloat(priceStr, 64)
					if err := tb.Trader.BuyToken(asset, amount, price); err != nil {
						tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка покупки: "+err.Error()))
						delete(tb.AwaitingBuyInput, update.Message.Chat.ID)
						delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
						continue
					}

					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Успешно куплено %.2f %s по цене $%.2f.", amount, asset, price)))
					delete(tb.AwaitingBuyInput, update.Message.Chat.ID)
					delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
					continue
				}

				// Проверяем, было ли это продажей
				if tb.AwaitingSellInput[update.Message.Chat.ID] {
					priceStr, err := tb.getPriceWithRetries(asset)
					if err != nil {
						tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка получения цены: "+err.Error()))
						delete(tb.AwaitingSellInput, update.Message.Chat.ID)
						delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
						continue
					}

					price, _ := strconv.ParseFloat(priceStr, 64)
					if _, err := tb.Trader.SellToken(asset, amount, price); err != nil {
						tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка продажи: "+err.Error()))
						delete(tb.AwaitingSellInput, update.Message.Chat.ID)
						delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
						continue
					}

					tb.Bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Успешно продано %.2f %s по цене $%.2f.", amount, asset, price)))
					delete(tb.AwaitingSellInput, update.Message.Chat.ID)
					delete(tb.AwaitingAmountInput, update.Message.Chat.ID)
					continue
				}
			}
		}
	}
}

// Функция для получения цены с повторными попытками
func (tb *TelegramBot) getPriceWithRetries(symbol string) (string, error) {
	const maxRetries = 3
	const retryDelay = 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		price, err := okx.GetCurrentPrice(symbol)
		if err != nil {
			if err.Error() == "429 Too Many Requests" {
				time.Sleep(retryDelay)
				continue
			}
			return "", err
		}
		return price, nil
	}
	return "", fmt.Errorf("не удалось получить цену после нескольких попыток")
}

// Функция для проверки, является ли актив допустимым
func isValidAsset(symbol string) bool {
	validAssets := []string{"BTC-USDT", "ETH-USDT", "XRP-USDT", "TON-USDT", "LTC-USDT",
		"BCH-USDT", "ADA-USDT", "DOT-USDT", "SOL-USDT", "DOGE-USDT"}
	for _, asset := range validAssets {
		if asset == symbol {
			return true
		}
	}
	return false
}
