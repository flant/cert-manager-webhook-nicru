FROM golang:1.25-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
ARG versionflags=""
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o webhook -ldflags "-w -s -extldflags '-static' ${versionflags}" .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /src/webhook /usr/local/bin/webhook
ENTRYPOINT ["webhook"]
