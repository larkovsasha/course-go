FROM golang:1.27-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/trip-service ./cmd/trip-service/main.go

FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app/
COPY --from=builder /app/trip-service .

ENTRYPOINT ["/app/trip-service"]