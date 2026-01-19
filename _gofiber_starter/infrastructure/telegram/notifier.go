package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gofiber-template/domain/ports"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

// TelegramNotifier - Telegram implementation of NotifierPort
type TelegramNotifier struct {
	settingService services.SettingService
	httpClient     *http.Client
}

// NewTelegramNotifier สร้าง TelegramNotifier
func NewTelegramNotifier(settingService services.SettingService) ports.NotifierPort {
	return &TelegramNotifier{
		settingService: settingService,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsEnabled ตรวจสอบว่าเปิดใช้งานการแจ้งเตือนหรือไม่
func (n *TelegramNotifier) IsEnabled() bool {
	ctx := context.Background()
	return n.settingService.GetBool(ctx, "alert", "enabled", false)
}

// getConfig ดึงค่า config จาก settings
func (n *TelegramNotifier) getConfig(ctx context.Context) (botToken, chatID string, err error) {
	botToken, err = n.settingService.Get(ctx, "alert", "telegram_bot_token")
	if err != nil || botToken == "" {
		return "", "", fmt.Errorf("telegram_bot_token not configured")
	}

	chatID, err = n.settingService.Get(ctx, "alert", "telegram_chat_id")
	if err != nil || chatID == "" {
		return "", "", fmt.Errorf("telegram_chat_id not configured")
	}

	return botToken, chatID, nil
}

// sendMessage ส่งข้อความไปยัง Telegram
func (n *TelegramNotifier) sendMessage(ctx context.Context, message string) error {
	if !n.IsEnabled() {
		logger.InfoContext(ctx, "Telegram notification disabled, skipping")
		return nil
	}

	botToken, chatID, err := n.getConfig(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Telegram config error", "error", err)
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to send Telegram message", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.ErrorContext(ctx, "Telegram API error", "status", resp.StatusCode)
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	logger.InfoContext(ctx, "Telegram notification sent successfully")
	return nil
}

// SendDLQAlert ส่งแจ้งเตือนเมื่อวิดีโอเข้า DLQ
func (n *TelegramNotifier) SendDLQAlert(ctx context.Context, notification *ports.DLQNotification) error {
	if !n.settingService.GetBool(ctx, "alert", "on_dlq", true) {
		logger.InfoContext(ctx, "DLQ notification disabled")
		return nil
	}

	message := fmt.Sprintf(`🚨 <b>วิดีโอเข้า Dead Letter Queue</b>

📹 <b>%s</b>
📝 Code: <code>%s</code>
🔄 Retry attempts: %d
⚙️ Stage: %s
👷 Worker: %s

❌ <b>Error:</b>
<pre>%s</pre>

⏰ Failed at: %s`,
		escapeHTML(notification.Title),
		notification.VideoCode,
		notification.Attempts,
		notification.Stage,
		notification.WorkerID,
		escapeHTML(truncateString(notification.Error, 500)),
		notification.FailedAt,
	)

	return n.sendMessage(ctx, message)
}

// SendTranscodeCompleteAlert ส่งแจ้งเตือนเมื่อ transcode สำเร็จ
func (n *TelegramNotifier) SendTranscodeCompleteAlert(ctx context.Context, videoCode, title string) error {
	if !n.settingService.GetBool(ctx, "alert", "on_transcode_complete", false) {
		return nil
	}

	message := fmt.Sprintf(`✅ <b>แปลงวิดีโอสำเร็จ</b>

📹 <b>%s</b>
📝 Code: <code>%s</code>`,
		escapeHTML(title),
		videoCode,
	)

	return n.sendMessage(ctx, message)
}

// SendTranscodeFailAlert ส่งแจ้งเตือนเมื่อ transcode ล้มเหลว
func (n *TelegramNotifier) SendTranscodeFailAlert(ctx context.Context, videoCode, title, errorMsg string) error {
	if !n.settingService.GetBool(ctx, "alert", "on_transcode_fail", true) {
		return nil
	}

	message := fmt.Sprintf(`⚠️ <b>แปลงวิดีโอล้มเหลว</b>

📹 <b>%s</b>
📝 Code: <code>%s</code>

❌ <b>Error:</b>
<pre>%s</pre>`,
		escapeHTML(title),
		videoCode,
		escapeHTML(truncateString(errorMsg, 500)),
	)

	return n.sendMessage(ctx, message)
}

// SendWorkerOfflineAlert ส่งแจ้งเตือนเมื่อ worker offline
func (n *TelegramNotifier) SendWorkerOfflineAlert(ctx context.Context, workerID, hostname, lastSeen string) error {
	if !n.settingService.GetBool(ctx, "alert", "on_worker_offline", true) {
		return nil
	}

	message := fmt.Sprintf(`🔴 <b>Worker Offline</b>

🖥️ <b>%s</b>
🆔 ID: <code>%s</code>
⏰ Last seen: %s`,
		escapeHTML(hostname),
		workerID,
		lastSeen,
	)

	return n.sendMessage(ctx, message)
}

// escapeHTML escape HTML special characters for Telegram
func escapeHTML(s string) string {
	replacer := map[string]string{
		"&":  "&amp;",
		"<":  "&lt;",
		">":  "&gt;",
	}
	result := s
	for old, new := range replacer {
		result = replaceAll(result, old, new)
	}
	return result
}

// replaceAll replaces all occurrences
func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}

// truncateString truncates string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
