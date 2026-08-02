FROM golang:1.22.2-alpine AS build

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gymtracker-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gymtracker-migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gymtracker-dbseed ./cmd/dbseed

FROM scratch AS migrate

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/gymtracker-migrate /gymtracker-migrate
COPY backend/migrations /migrations

USER 65532:65532
ENTRYPOINT ["/gymtracker-migrate"]

FROM scratch AS dbseed

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/gymtracker-dbseed /gymtracker-dbseed

USER 65532:65532
ENTRYPOINT ["/gymtracker-dbseed"]

FROM scratch AS api

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/gymtracker-api /gymtracker-api

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/gymtracker-api"]
