ARG GO_VERSION
FROM golang:${GO_VERSION}-alpine3.19 as build

COPY . /app

RUN \
    cd /app && \
    mkdir dist && \
    go build -trimpath -ldflags="-s -w" -v -o dist/

FROM alpine:3.19

COPY --from=build /app/dist/gitea-actions-cache-server /gitea-actions-cache-server

ENTRYPOINT ["/gitea-actions-cache-server"]
