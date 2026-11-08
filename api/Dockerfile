FROM golang:1.27-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/beer-menu-api ./cmd/beer-menu-api

FROM scratch
COPY --from=build /out/beer-menu-api /beer-menu-api
# Non-root default only; the deployer may run it as any non-root UID/GID.
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s CMD ["/beer-menu-api","healthcheck"]
ENTRYPOINT ["/beer-menu-api"]
