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

// broadcastPollResult отправляет результат опроса (statusCode + кусок body) всем клиентам
// через WebSocket текстовый фрейм (OpcodeText). Вызывается из горутины поллера.
func broadcastPollResult(statusCode int, body string) {
	// Ограничиваем длину body, чтобы не слать клиентам мегабайты HTML.
	const maxBodyPreview = 200
	if len(body) > maxBodyPreview {
		body = body[:maxBodyPreview] + "..."
	}
	msg := []byte(fmt.Sprintf(`{"status_code":%d,"body":%q}`, statusCode, body))

	hubMu.Lock()
	defer hubMu.Unlock()
	for conn := range hubConns {
		_ = conn.WriteMessage(gws.OpcodeText, msg) // тот же WriteMessage, что и в OnMessage
	}
}

func main() {
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

	ctx := context.Background()
	ch := StartPoller(ctx, "https://dzen.ru", 10*time.Second)

	// Горутина: читаем результаты поллера из канала и рассылаем их всем WS-клиентам.
	go func() {
		for r := range ch {
			if r.Err != nil {
				fmt.Println("poll error:", r.Err)
				broadcastPollResult(0, "poll error: "+r.Err.Error()) // 0 = нет кода ответа
				continue
			}
			fmt.Println("status:", r.StatusCode)
			broadcastPollResult(r.StatusCode, string(r.Body))
		}
	}()

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
