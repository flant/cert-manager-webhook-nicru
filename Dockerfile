FROM golang:1.22.3-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
ENV GO111MODULE=on
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

# ------------------------------
FROM alpine:3.14.2
RUN apk add --no-cache ca-certificates
COPY --from=build /src/webhook /usr/local/bin/webhook
ENTRYPOINT ["webhook"]