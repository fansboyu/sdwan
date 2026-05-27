FROM golang:1.25-alpine AS build

WORKDIR /src
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=$GOPROXY
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/controller ./cmd/controller

FROM alpine:3.21

RUN adduser -D -H -s /sbin/nologin sdwan
USER sdwan

COPY --from=build /out/controller /controller
EXPOSE 8080

ENTRYPOINT ["/controller"]
