FROM scratch

COPY dist/gitea-actions-cache-server /go/bin/gitea-actions-cache-server

ENTRYPOINT ["/go/bin/gitea-actions-cache-server"]
