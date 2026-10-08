# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM docker.io/library/golang:1.26.9-bookworm@sha256:d9c68c2c51161e12fd77e4c6320687c9cd86e1af1e3ad6e6cd63ff970641453c AS build
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum* ./
RUN go mod download
COPY apps/api .
RUN CGO_ENABLED=0 go build -o /out/ ./cmd/server ./cmd/schema ./cmd/migrate ./cmd/spicedb-runtime-proxy ./cmd/audit-retention ./cmd/account-deletion-records ./cmd/policy-publish
RUN mkdir -p /out/deletion-journal && chmod 0700 /out/deletion-journal
FROM registry.access.redhat.com/hi/static:latest-1788855983@sha256:20f419d12511f96524d9b9bb092ef5066d6bacee7ed45d7c528bef62f6d48f74
COPY --from=build --chown=65532:65532 /out/deletion-journal /var/lib/hourpaths-deletion
COPY --from=build /out/server /app
COPY --from=build /out/schema /schema
COPY --from=build /out/migrate /migrate
COPY --from=build /out/spicedb-runtime-proxy /spicedb-runtime-proxy
COPY --from=build /out/audit-retention /audit-retention
COPY --from=build /out/account-deletion-records /account-deletion-records
COPY --from=build /out/policy-publish /policy-publish
ENTRYPOINT ["/app"]
