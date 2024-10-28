package okx

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Структура для ответа API
type PriceResponse struct {
	Result []struct {
		Symbol    string `json:"symbol"`
		LastPrice string `json:"last_price"`
	} `json:"result"`
}

// Пример структуры для хранения данных об активах
type Asset struct {
	Symbol string `json:"name"`
}

// GetAssets возвращает список из 10 популярных активов
func GetAssets() ([]string, error) {
	popularAssets := []string{
		"BTC-USDT", "ETH-USDT", "XRP-USDT", "LTC-USDT", "BCH-USDT",
		"TON-USDT", "DOT-USDT", "SOL-USDT", "DOGE-USDT", "LINK-USDT",
	}

	return popularAssets, nil
}

// Функция для получения текущей цены актива
func GetCurrentPrice(symbol string) (string, error) {
	url := fmt.Sprintf("https://www.okx.com/api/v5/market/ticker?instId=%s", symbol)
	log.Printf("Запрос к URL: %s", url) // Логируем URL

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Ошибка при выполнении запроса: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("Получен статус код: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка при запросе: %s", resp.Status)
	}

	// Структура для ответа API OKX
	var priceResponse struct {
		Code string `json:"code"`
		Data []struct {
			LastPrice string `json:"last"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&priceResponse); err != nil {
		log.Printf("Ошибка при декодировании JSON: %v", err)
		return "", err
	}

	if priceResponse.Code != "0" || len(priceResponse.Data) == 0 {
		return "", fmt.Errorf("не удалось найти цену для актива %s", symbol)
	}

	price := priceResponse.Data[0].LastPrice
	log.Printf("Получена цена: %s", price)
	return price, nil
}
