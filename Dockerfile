FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/docmind-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/docmind-seed ./cmd/seed && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/docmind-telegram ./cmd/telegram

FROM alpine:3.20
RUN apk add --no-cache ca-certificates poppler-utils && addgroup -S docmind && adduser -S -G docmind docmind
COPY --from=build --chown=docmind:docmind /out/docmind-api /usr/local/bin/docmind-api
COPY --from=build --chown=docmind:docmind /out/docmind-seed /usr/local/bin/docmind-seed
COPY --from=build --chown=docmind:docmind /out/docmind-telegram /usr/local/bin/docmind-telegram
USER docmind:docmind
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/docmind-api"]
