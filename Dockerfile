FROM golang:1.23 AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /tmp/copilot-api .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /tmp/copilot-api /usr/local/bin/copilot-api

EXPOSE 4141
VOLUME ["/root/.local/share/copilot-api"]

ENTRYPOINT ["copilot-api"]
CMD ["start", "--port", "4141"]
