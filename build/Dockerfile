FROM golang:1.21 AS build

ARG version

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -o ./pico main.go


FROM alpine:latest

COPY --from=build app/pico /bin/pico

ENTRYPOINT [ "/bin/pico" ]
