// Package service реализует usecase.API поверх портов из usecase.Deps.
package service

import (
	"sync"

	"orderissue/internal/usecase"
)

// maxWriteCycles — сколько раз подряд «поиск → создание» может закончиться
// неизвестным исходом, прежде чем сдаться. Каждый цикл внутри уже содержит
// повторы чтения; три цикла умещаются в дедлайн записи.
const maxWriteCycles = 3

// Service — сценарии ручек.
type Service struct {
	d usecase.Deps
	// orderLocks сериализует запись черновика по одному заказу: два одновременных
	// нажатия иначе оба не найдут черновик и оба его создадут.
	orderLocks sync.Map
}
