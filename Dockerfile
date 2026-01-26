FROM golang:1.25-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src
ENV GO111MODULE=on
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -s -extldflags "-static"' ./cmd/webhook

# ------------------------------
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /src/webhook /usr/local/bin/webhook
USER 65532:65532
ENTRYPOINT ["webhook"]
