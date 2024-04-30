FROM alpine:3.19

COPY dist/gitea-actions-cache-server /gitea-actions-cache-server

ENTRYPOINT ["/gitea-actions-cache-server"]
