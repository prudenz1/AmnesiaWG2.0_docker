FROM golang:1.24 AS awg-build

RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
RUN git clone https://github.com/amnezia-vpn/amneziawg-go /src/amneziawg-go
WORKDIR /src/amneziawg-go
RUN go mod download && go mod verify
RUN CGO_ENABLED=1 go build -ldflags "-linkmode external -extldflags '-fno-PIC -static'" -v -o /usr/local/bin/amneziawg-go

FROM alpine:3.21 AS tools-build

RUN apk add --no-cache build-base bash linux-headers make git
RUN git clone https://github.com/amnezia-vpn/amneziawg-tools /src/amneziawg-tools
WORKDIR /src/amneziawg-tools/src
RUN make
RUN mkdir -p /out && cp wg /out/awg && cp wg-quick/linux.bash /out/awg-quick

FROM golang:1.24 AS api-build

WORKDIR /src
COPY api/go.mod ./
COPY api/main.go ./
RUN go mod tidy
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /usr/local/bin/awg-api .

FROM alpine:3.21

RUN apk add --no-cache bash iproute2 iptables ip6tables ca-certificates coreutils grep sed

RUN mkdir -p /etc/amnezia/amneziawg /var/lib/amneziawg /run/amneziawg

COPY --from=awg-build /usr/local/bin/amneziawg-go /usr/local/bin/amneziawg-go
COPY --from=tools-build /out/awg /usr/local/bin/awg
COPY --from=tools-build /out/awg-quick /usr/local/bin/awg-quick
COPY --from=api-build /usr/local/bin/awg-api /usr/local/bin/awg-api
COPY entrypoint.sh /entrypoint.sh

RUN sed -i 's/\r$//' /entrypoint.sh && chmod +x /entrypoint.sh /usr/local/bin/awg /usr/local/bin/awg-quick /usr/local/bin/amneziawg-go /usr/local/bin/awg-api

EXPOSE 51820/udp 8080/tcp

ENTRYPOINT ["/entrypoint.sh"]
