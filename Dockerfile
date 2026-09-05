FROM golang:1.26.4-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/b3-data-hub ./cmd

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/b3-data-hub /app/b3-data-hub
COPY docker/crontab /etc/crontabs/root
COPY config/trading-calendar.json /app/config/trading-calendar.json

RUN mkdir -p /app/data

ENV TZ=America/Sao_Paulo
ENV DATA_DIR=/app/data
ENV TRADING_CALENDAR_PATH=/app/config/trading-calendar.json

CMD ["crond", "-f", "-l", "2"]
