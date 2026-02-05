package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lxzan/gws"
)

const (
	PingInterval = 5 * time.Second
	PingWait     = 10 * time.Second
)

// --- Hub для рассылки результата поллера всем подключённым клиентам ---
// hubMu защищает hubConns от одновременного доступа из разных горутин
// (поллер, OnOpen, OnClose вызываются асинхронно).
var (
	hubMu    sync.Mutex
	hubConns = make(map[*gws.Conn]struct{}) // множество соединений; struct{} не занимает памяти
)

// broadcastPollResult отправляет результат опроса (url, statusCode, body) всем клиентам
// через WebSocket текстовый фрейм (OpcodeText). url — какой сайт опрашивали, чтобы клиент различал.
func broadcastPollResult(url string, statusCode int, body string) {
	const maxBodyPreview = 200
	if len(body) > maxBodyPreview {
		body = body[:maxBodyPreview] + "..."
	}
	msg := []byte(fmt.Sprintf(`{"url":%q,"status_code":%d,"body":%q}`, url, statusCode, body))

	hubMu.Lock()
	defer hubMu.Unlock()
	for conn := range hubConns {
		_ = conn.WriteMessage(gws.OpcodeText, msg) // тот же WriteMessage, что и в OnMessage
	}
}

func main() {
	// Загружаем переменные окружения из .env файла при старте программы.
	if err := loadEnv(".env"); err != nil {
		fmt.Println("Warning: не удалось загрузить .env:", err)
		// Продолжаем работу — возможно, переменные заданы в системе
	}

	SendTelegramMessage("test")

	upgrader := gws.NewUpgrader(&Handler{}, &gws.ServerOption{

		ParallelEnabled:   true,                                  // Parallel message processing
		Recovery:          gws.Recovery,                          // Exception recovery
		PermessageDeflate: gws.PermessageDeflate{Enabled: false}, // Enable compression
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			fmt.Println("WS Origin:", r.Header.Get("Origin"))
			return true // для разработки разрешаем всё
		},
	})
	http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		socket, err := upgrader.Upgrade(w, r)
		if err != nil {
			// покажет реальную причину: 403, handshake error, и т.п.
			fmt.Println("Upgrade error:", err, "Origin:", r.Header.Get("Origin"))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		go socket.ReadLoop()
	})

	// context.Context — это интерфейс в Go для передачи сигналов отмены, дедлайнов и значений
	// между горутинами и функциями. Это стандартный способ управления жизненным циклом операций.
	//
	// context.Background() — создаёт "пустой" контекст, который никогда не отменяется.
	// Используется как корневой контекст для долгоживущих операций (например, поллер работает
	// пока работает программа). Если бы нужна была возможность остановить поллер по требованию,
	// использовали бы context.WithCancel() и вызывали cancel() для остановки.
	ctx := context.Background()
	pollInterval := 10 * time.Second

	// Список сайтов для опроса — каждый опрашивается своим поллером независимо.
	pollTargets := []string{
		"https://dzen.ru",
		"https://ya.ru",
	}

	// Для каждого URL запускаем отдельный поллер и горутину, которая рассылает результаты клиентам.
	for _, url := range pollTargets {
		url := url // для замыкания в горутине
		ch := StartPoller(ctx, url, pollInterval)
		go func() {
			for r := range ch {
				if r.Err != nil {
					fmt.Println("poll error [", url, "]:", r.Err)
					broadcastPollResult(url, 0, "poll error: "+r.Err.Error())
					continue
				}
				fmt.Println(url, "status:", r.StatusCode)
				broadcastPollResult(url, r.StatusCode, string(r.Body))
			}
		}()
	}

	http.ListenAndServe(":8081", nil)
}

// ---------------------------------------------------------
type Handler struct{}

func (c *Handler) OnOpen(socket *gws.Conn) {
	// Регистрируем соединение в hub, чтобы поллер мог слать ему broadcastPollResult.
	hubMu.Lock()
	hubConns[socket] = struct{}{}
	hubMu.Unlock()
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))

	// серверный ping-цикл
	go func() {
		ticker := time.NewTicker(PingInterval)
		defer ticker.Stop()

		for range ticker.C {
			// если клиент умер/сеть умерла — WritePing вернёт ошибку, выходим
			if err := socket.WritePing(nil); err != nil {
				return
			}
		}
	}()
}

func (c *Handler) OnClose(socket *gws.Conn, err error) {
	// Убираем соединение из hub, чтобы не писать в закрытый сокет и не держать ссылку.
	hubMu.Lock()
	delete(hubConns, socket)
	hubMu.Unlock()
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	// _ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	// _ = socket.WritePong(nil)
}

func (c *Handler) OnPong(socket *gws.Conn, payload []byte) {
	// получили pong -> продлеваем дедлайн
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	fmt.Println(message)
	// На всякий случай можно продлевать дедлайн и по любому трафику
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
