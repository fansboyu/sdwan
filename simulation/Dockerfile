FROM golang:1.25-alpine AS build

WORKDIR /src
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=$GOPROXY
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/sdwan-agent ./cmd/agent
RUN CGO_ENABLED=0 go build -o /out/sdwan-relay-agent ./cmd/relay-agent
RUN CGO_ENABLED=0 go build -o /out/sdwan-lab-seed ./simulation

FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl iproute2 iptables iputils jq wireguard-tools
RUN mkdir -p /srv
COPY --from=build /out/sdwan-agent /usr/local/bin/sdwan-agent
COPY --from=build /out/sdwan-relay-agent /usr/local/bin/sdwan-relay-agent
COPY --from=build /out/sdwan-lab-seed /usr/local/bin/sdwan-lab-seed
COPY simulation/agent-entrypoint.sh /usr/local/bin/agent-entrypoint.sh
COPY simulation/hq-index.html /srv/index.html
RUN chmod 0755 /usr/local/bin/agent-entrypoint.sh
ENTRYPOINT ["/usr/local/bin/agent-entrypoint.sh"]
