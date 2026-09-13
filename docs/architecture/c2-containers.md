# C2. Контейнеры

```mermaid
C4Container
  title C2. Из каких частей состоит сервис
  Person(clerk, "Кладовщик")
  Person(reviewer, "Проверяющий")

  System_Boundary(sys, "Сервис выдачи заказов") {
    Container(page, "Страница кладовщика", "HTML, CSS, JS", "Выбор заказа, форма получателя, печать и запись черновика")
    Container(api, "HTTP-сервис", "Go, net/http, порт 3000", "API v1, алиас для автопроверки, PDF, Swagger UI; отдаёт страницу")
  }

  System_Ext(onec, "1С, эмулятор", "OData")

  Rel(clerk, page, "Работает")
  Rel(page, api, "Вызывает API v1", "JSON")
  Rel(reviewer, api, "POST /api/documents/preview", "JSON")
  Rel(api, onec, "Чтение и запись", "OData")
```

| Контейнер | Запуск | Состояние |
|---|---|---|
| HTTP-сервис | `docker compose up` или `go run ./cmd/server` | ничего не хранит: данные читаются из 1С на каждый запрос |
| Страница | вшита в сервис, открывается по `/` | в браузере, до перезагрузки |
