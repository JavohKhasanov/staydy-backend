# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

# --- API server (Huma/OpenAPI + Echo) ---
FROM gcr.io/distroless/static-debian12 AS api
COPY --from=build /out/api /api
ENTRYPOINT ["/api"]
