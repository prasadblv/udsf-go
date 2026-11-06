# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/udsf ./cmd/api

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/udsf /udsf
# TLS cert/key are mounted at runtime (see docker-compose.yml / generate-cert.sh).
ENV PORT=9443 \
    GIN_MODE=release \
    UDSF_TLS_CERT=/certs/server.pem \
    UDSF_TLS_KEY=/certs/server.key
EXPOSE 9443
USER nonroot:nonroot
ENTRYPOINT ["/udsf"]
