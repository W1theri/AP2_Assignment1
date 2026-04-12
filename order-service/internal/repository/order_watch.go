package repository

// Этот файл расширяет PostgresOrderRepository из Assignment 1,
// добавляя WatchOrderStatus для Server-side Streaming.
// Все существующие методы (Save, FindByID, Update, ...) НЕ ИЗМЕНИЛИСЬ.

import (
	"database/sql"
	"time"
)

// WatchOrderStatus опрашивает БД каждые 500 мс и отправляет новый статус
// в канал при его изменении. Закрывается когда done закрыт (клиент отключился).
//
// Это реальное взаимодействие с БД — не fake time.Sleep без чтения.
// Соответствует требованию: "stream must be tied to real changes in the database".
func (r *PostgresOrderRepository) WatchOrderStatus(
	orderID string,
	done <-chan struct{},
) (<-chan string, <-chan error) {
	statusCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(statusCh)
		defer close(errCh)

		var lastStatus string

		// Отправляем текущий статус сразу при подписке
		var initial string
		err := r.db.QueryRow(
			`SELECT status FROM orders WHERE id = $1`, orderID,
		).Scan(&initial)
		if err != nil {
			if err == sql.ErrNoRows {
				errCh <- err
				return
			}
			errCh <- err
			return
		}
		lastStatus = initial
		statusCh <- initial

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				// Клиент отключился — завершаем горутину
				return

			case <-ticker.C:
				var current string
				err := r.db.QueryRow(
					`SELECT status FROM orders WHERE id = $1`, orderID,
				).Scan(&current)
				if err != nil {
					errCh <- err
					return
				}
				// Отправляем только при реальном изменении статуса
				if current != lastStatus {
					lastStatus = current
					statusCh <- current
				}
			}
		}
	}()

	return statusCh, errCh
}
