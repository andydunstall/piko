FROM golang:1.21 AS build

ARG version

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -o ./piko main.go


FROM alpine:latest

COPY --from=build app/piko /bin/piko

ENTRYPOINT [ "/bin/piko" ]
