package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	configURL = "https://raw.githubusercontent.com/igareck/vpn-configs-for-russia/main/WHITE-CIDR-RU-all.txt"
)

func main() {
	// Получаем токен из переменной окружения
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
	}

	// Инициализируем бота
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Настраиваем получение обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Обрабатываем входящие сообщения
	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				log.Printf("Received /start from chat %d", update.Message.Chat.ID)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Используй /configs для получения VLESS конфигураций.")
				if _, err := bot.Send(msg); err != nil {
					log.Printf("Error sending start message: %v", err)
				}
			case "configs":
				log.Printf("Received /configs from chat %d", update.Message.Chat.ID)
				// Запускаем в горутине чтобы не блокировать обработку других сообщений
				go sendConfigs(bot, update.Message.Chat.ID)
			}
		}
	}
}

// sendConfigs получает конфигурации с GitHub и отправляет их пользователю
func sendConfigs(bot *tgbotapi.BotAPI, chatID int64) {
	log.Printf("Starting sendConfigs for chat %d", chatID)

	// Делаем HTTP запрос к GitHub
	resp, err := http.Get(configURL)
	if err != nil {
		log.Printf("Error fetching configs: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Ошибка при получении конфигураций")
		bot.Send(msg)
		return
	}
	defer resp.Body.Close()

	log.Printf("HTTP response status: %d", resp.StatusCode)

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Ошибка чтения ответа")
		bot.Send(msg)
		return
	}

	log.Printf("Response body length: %d bytes", len(body))

	// Ищем все vless:// конфигурации (могут быть многострочными из-за переносов)
	// Убираем переносы строк и ищем паттерн vless://...#
	content := strings.ReplaceAll(string(body), "\n", " ")
	re := regexp.MustCompile(`vless://[^\s]+`)
	matches := re.FindAllString(content, -1)

	log.Printf("Найдено конфигураций: %d", len(matches))
	if len(matches) > 0 {
		log.Printf("Первая конфигурация: %s", matches[0][:50])
	}

	if len(matches) == 0 {
		log.Printf("No configs found in response")
		msg := tgbotapi.NewMessage(chatID, "Конфигурации не найдены")
		bot.Send(msg)
		return
	}

	log.Printf("Sending %d configs to chat %d", len(matches), chatID)

	// Отправляем конфигурации частями по 10 штук (конфигурации длинные, лимит Telegram 4096 символов)
	var batch []string
	for i, cfg := range matches {
		batch = append(batch, cfg)
		if len(batch) == 10 || i == len(matches)-1 {
			text := strings.Join(batch, "\n\n")
			// Проверяем длину сообщения
			if len(text) > 4000 {
				// Если слишком длинное, отправляем по одной конфигурации
				for _, singleCfg := range batch {
					msg := tgbotapi.NewMessage(chatID, singleCfg)
					bot.Send(msg)
				}
			} else {
				msg := tgbotapi.NewMessage(chatID, text)
				bot.Send(msg)
			}
			batch = nil
		}
	}

	// Отправляем итоговое сообщение
	finalMsg := tgbotapi.NewMessage(chatID, "✅ Все конфигурации отправлены!")
	bot.Send(finalMsg)
}
