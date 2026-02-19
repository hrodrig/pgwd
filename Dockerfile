FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pgwd ./cmd/pgwd

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=build /pgwd /pgwd
ENTRYPOINT ["/pgwd"]
