# --- Стадия сборки -----------------------------------------------------------
# go-sqlite3 требует CGO, поэтому нужен компилятор C (musl-dev в Alpine).
FROM golang:1.25-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# Сначала зависимости — слой кэшируется, пока go.mod/go.sum не меняются.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO включён ради sqlite; статическая линковка, чтобы бинарник не тянул
# системные библиотеки и работал в минимальном финальном образе.
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags '-s -w -extldflags "-static"' \
    -o /out/tasktracker ./cmd/tasktracker

# --- Финальный образ ---------------------------------------------------------
FROM alpine:3.20

# ca-certificates — для HTTPS к api.telegram.org; tzdata подстрахует таймзоны.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 app

# Каталог под базу данных (монтируется как volume).
RUN mkdir -p /data && chown app:app /data

COPY --from=build /out/tasktracker /usr/local/bin/tasktracker

USER app
WORKDIR /data

# Путь к БД по умолчанию указывает на volume.
ENV DB_PATH=/data/tasks.db

ENTRYPOINT ["tasktracker"]
