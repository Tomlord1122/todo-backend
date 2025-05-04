FROM golang:1.24.2 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary for Linux, suitable for Alpine
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/api/main.go

FROM alpine:3.20.1 AS prod
WORKDIR /app
COPY --from=build /app/main /app/main
RUN chmod +x /app/main
EXPOSE ${PORT}
CMD ["./main"]


