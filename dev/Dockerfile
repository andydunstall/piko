FROM golang:1.21 AS build

ARG version

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -o ./pico main.go

FROM alpine:latest

ENV PORT 8080

COPY --from=build app/pico /usr/bin/pico
