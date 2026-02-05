package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadEnv читает файл .env и устанавливает переменные окружения.
// Формат файла: KEY=VALUE (по одной на строку, пустые строки и строки с # игнорируются).
// Вызывается в начале main() чтобы автоматически загрузить настройки.
func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("не удалось открыть %s: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		// Пропускаем пустые строки и комментарии
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Разбиваем на KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // пропускаем некорректные строки
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" && value != "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
