FROM golang:1.24.1 AS build

ARG version

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-X github.com/andydunstall/piko/pkg/build.Version=$version" -o ./piko main.go


FROM alpine:latest

COPY --from=build app/piko /bin/piko

ENTRYPOINT [ "/bin/piko" ]
