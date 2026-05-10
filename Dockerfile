FROM golang:1.26.3-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mpal-mcp ./cmd/mpal-mcp

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
LABEL org.opencontainers.image.source="https://github.com/revrost/mpal-cli"
LABEL org.opencontainers.image.description="MarketPal MCP capability server"
LABEL io.modelcontextprotocol.server.name="io.github.revrost/mpal"

COPY --from=build /out/mpal-mcp /usr/local/bin/mpal-mcp
ENTRYPOINT ["mpal-mcp"]
