FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:alpine
COPY --from=build /src/dist /usr/share/nginx/html
COPY nginx/default.conf /etc/nginx/conf.d/default.conf
# Must stay executable in git: the base entrypoint skips non-executable scripts.
COPY nginx/00-refuse-root.sh /docker-entrypoint.d/00-refuse-root.sh
# Non-root default only; the deployer may run it as any non-root UID/GID.
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s CMD wget -q --spider http://127.0.0.1:8080/ || exit 1
