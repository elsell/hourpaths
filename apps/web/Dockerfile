# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM registry.access.redhat.com/hi/nodejs:24.18.1-builder-1790203435@sha256:1cba44643685f640a135576a0100bb8a066545c629d99f9eac5a94db3a7eb232 AS build
ENV NPM_CONFIG_PREFIX=/tmp/npm-global PATH=/tmp/npm-global/bin:$PATH
WORKDIR /app
RUN npm install --global pnpm@11.0.7
COPY --chown=65532:0 package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY --chown=65532:0 patches patches
COPY --chown=65532:0 apps/web/package.json apps/web/package.json
COPY --chown=65532:0 apps/mobile/package.json apps/mobile/package.json
COPY --chown=65532:0 packages/api-client/package.json packages/api-client/package.json
COPY --chown=65532:0 packages/client-core/package.json packages/client-core/package.json
COPY --chown=65532:0 packages/i18n/package.json packages/i18n/package.json
RUN pnpm install --frozen-lockfile
COPY --chown=65532:0 apps/web apps/web
COPY --chown=65532:0 packages/api-client packages/api-client
COPY --chown=65532:0 packages/client-core packages/client-core
COPY --chown=65532:0 packages/i18n packages/i18n
RUN pnpm --dir apps/web build
RUN pnpm --filter @hourpaths/web deploy --legacy --prod /app/prod

FROM registry.access.redhat.com/hi/nodejs:24.18.1-1790203435@sha256:5c2430ca1a1253f680fb9b654c18071bf52c94b336c4a60f5f779dedd9b743b7
ENV NODE_ENV=production HOURPATHS_APP_ENV=production HOST=0.0.0.0 PORT=3000
WORKDIR /app
USER 0
# The runtime only needs Node. Remove the base image's bundled npm tree so
# package-manager dependencies are not shipped or scanned with the service.
RUN find / -xdev -path '*/node_modules/npm' -prune -exec rm -rf '{}' '+' 2>/dev/null || true
COPY --from=build --chown=65532:0 /app/prod ./
USER 65532
EXPOSE 3000
CMD ["node", "build"]
