package trader

import (
	"fmt"
	"strconv"

	"github.com/VadimBorzenkov/TradeSimulatorBot/okx"
)

// Investment хранит данные о конкретной инвестиции в токен
type Investment struct {
	Token    string
	Amount   float64
	BuyPrice float64
}

// Balance содержит информацию о текущем состоянии инвестиций
type Balance struct {
	Investments []Investment
	TotalValue  float64 // Общая стоимость в долларах
}

// Trader управляет капиталом и выполняет торговые операции
type Trader struct {
	Capital     float64
	Investments []Investment
}

// BuyToken выполняет покупку токена по текущей цене
func (t *Trader) BuyToken(token string, amount float64, price float64) error {
	if amount > t.Capital {
		return fmt.Errorf("недостаточно капитала для покупки")
	}

	investment := Investment{
		Token:    token,
		Amount:   amount,
		BuyPrice: price,
	}

	// Добавляем инвестицию и уменьшаем капитал
	t.Investments = append(t.Investments, investment)
	t.Capital -= amount

	return nil
}

// SellToken выполняет продажу токена
func (t *Trader) SellToken(token string, amount float64, currentPrice float64) (float64, error) {
	for i, investment := range t.Investments {
		if investment.Token == token {
			if amount > investment.Amount {
				return 0, fmt.Errorf("недостаточно токенов для продажи")
			}

			// Рассчитываем прибыль
			profit := amount * (currentPrice / investment.BuyPrice)

			// Возвращаем прибыль в капитал и обновляем инвестицию
			t.Capital += profit
			investment.Amount -= amount

			// Если инвестиция полностью продана, удаляем её
			if investment.Amount == 0 {
				t.Investments = append(t.Investments[:i], t.Investments[i+1:]...)
			}

			return profit, nil
		}
	}
	return 0, fmt.Errorf("инвестиция в токен %s не найдена", token)
}

// ExecuteGridStrategy выполняет сеточную стратегию покупки/продажи
func (t *Trader) ExecuteGridStrategy(token string, priceDropPercent, priceRisePercent, amount float64) error {
	currentPrice, err := okx.GetCurrentPrice(token + "-USDT")
	if err != nil {
		return err
	}

	price, _ := strconv.ParseFloat(currentPrice, 64)

	// Покупка при падении цены
	for _, investment := range t.Investments {
		if investment.Token == token && price <= investment.BuyPrice*(1-priceDropPercent/100) {
			return t.BuyToken(token, amount, price) // Например, покупаем на указанное количество
		}
	}

	// Продажа при росте цены
	for _, investment := range t.Investments {
		if investment.Token == token && price >= investment.BuyPrice*(1+priceRisePercent/100) {
			_, err := t.SellToken(token, amount, price) // Продажа указанного количества
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *Trader) GetBalance() (*Balance, error) {
	if len(t.Investments) == 0 {
		return &Balance{
			Investments: []Investment{},
			TotalValue:  100.00,
		}, nil
	}

	totalValue := 0.0
	for _, investment := range t.Investments {
		totalValue += investment.Amount * investment.BuyPrice
	}

	return &Balance{
		Investments: t.Investments,
		TotalValue:  totalValue,
	}, nil
}

func (t *Trader) GetCapital() float64 {
	return t.Capital
}
