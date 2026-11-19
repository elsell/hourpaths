# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM registry.access.redhat.com/hi/nodejs:26.13.1-builder-1794592837@sha256:8dd0c730a05feb7e7419833775bf20cab39ce70a3dbc156faa2c6160f8297e12 AS build
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

FROM registry.access.redhat.com/hi/nodejs:26.13.1-fips-builder-1794592837@sha256:06bd282688af0539000a9856620d959cc98a81cb6441be2da3dde5480620eced
ENV NODE_ENV=production HOURPATHS_APP_ENV=production HOST=0.0.0.0 PORT=3000
WORKDIR /app
USER 0
# The runtime only needs Node. Remove the base image's bundled npm tree so
# package-manager dependencies are not shipped or scanned with the service.
RUN find /usr /opt -xdev \( -path '*/node_modules/npm' -o -path '*/node_modules_*/npm' \) -prune -exec rm -rf '{}' '+'
COPY --from=build --chown=65532:0 /app/prod ./
# The base image also carries a few package-manager dependencies outside npm's
# own tree. They are not needed by the service and must not ship in its image.
RUN for package in undici tar ip-address pacote postcss-selector-parser; do \
      find / -xdev -path "*/node_modules/${package}" ! -path '/app/prod/*' -prune -exec rm -rf '{}' '+' 2>/dev/null || true; \
    done
USER 65532
EXPOSE 3000
CMD ["node", "build"]
