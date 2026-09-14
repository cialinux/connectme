# syntax=docker/dockerfile:1.7
FROM docker.io/library/golang:1.24.6-alpine3.22 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/connectme ./cmd/connectme-server && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/connectme-migrate ./cmd/connectme-migrate && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/connectmectl ./cmd/connectmectl
FROM docker.io/library/alpine:3.22.1 AS server
# Match official guacd UID for private shared transfer directories.
RUN addgroup -S -g 1000 connectme && adduser -S -D -H -u 1000 -G connectme connectme
COPY --from=build /out/connectme /connectme
COPY --from=build /out/connectmectl /connectmectl
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/connectme"]
FROM docker.io/library/alpine:3.22.1 AS migrate
RUN addgroup -S -g 10001 connectme && adduser -S -D -H -u 10001 -G connectme connectme
COPY --from=build /out/connectme-migrate /connectme-migrate
USER 10001:10001
ENTRYPOINT ["/connectme-migrate"]
