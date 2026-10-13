# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM docker.io/library/golang:1.26.9-bookworm@sha256:d9c68c2c51161e12fd77e4c6320687c9cd86e1af1e3ad6e6cd63ff970641453c AS build
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum* ./
RUN go mod download
COPY apps/api .
RUN CGO_ENABLED=0 go build -o /out/ ./cmd/server ./cmd/schema ./cmd/migrate ./cmd/spicedb-runtime-proxy ./cmd/audit-retention ./cmd/account-deletion-records ./cmd/policy-publish ./cmd/account-enforce
RUN mkdir -p /out/deletion-journal && chmod 0700 /out/deletion-journal
FROM registry.access.redhat.com/hi/static:latest-1791331654@sha256:85fafe17d51d330089d4f39a9716bcc0eacfbda0c15ecb39f669cbfcd815ff8b
COPY --from=build --chown=65532:65532 /out/deletion-journal /var/lib/hourpaths-deletion
COPY --from=build /out/server /app
COPY --from=build /out/schema /schema
COPY --from=build /out/migrate /migrate
COPY --from=build /out/spicedb-runtime-proxy /spicedb-runtime-proxy
COPY --from=build /out/audit-retention /audit-retention
COPY --from=build /out/account-deletion-records /account-deletion-records
COPY --from=build /out/policy-publish /policy-publish
COPY --from=build /out/account-enforce /account-enforce
ENTRYPOINT ["/app"]
