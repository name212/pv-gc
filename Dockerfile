FROM golang:1.19 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /controller

FROM alpine:3.14.10

COPY --from=build-stage /controller /controller

CMD ["/controller"]