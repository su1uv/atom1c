FROM --platform=$BUILDPLATFORM golang:1.27.1-bookworm AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/atom1c . \
    && install -d -m 0700 -o 10001 -g 10001 /out/data /out/home/atom1c

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY container/passwd /etc/passwd
COPY container/group /etc/group
COPY --from=build --chown=10001:10001 /out/atom1c /usr/local/bin/atom1c
COPY --from=build --chown=10001:10001 /out/data /data
COPY --from=build --chown=10001:10001 /out/home/atom1c /home/atom1c

ENV HOME=/home/atom1c \
    GOOSE_DBSTRING=/data/atom1c.db \
    ATOM1C_SSH_ADDR=0.0.0.0:23234 \
    ATOM1C_AUTHORIZED_KEYS=/run/secrets/authorized_keys \
    ATOM1C_REFRESH_INTERVAL=15m \
    XDG_DATA_HOME=/data

USER 10001:10001
EXPOSE 23234
STOPSIGNAL SIGINT
ENTRYPOINT ["/usr/local/bin/atom1c"]
