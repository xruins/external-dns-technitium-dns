# Stage 1 – Build the binary.
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Cache dependency downloads separately from source changes.
COPY go.mod ./
RUN go mod download

COPY . .

# Produce a fully static binary suitable for a scratch/distroless image.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /external-dns-technitium-dns ./cmd/main.go

# Stage 2 – Minimal runtime image (distroless).
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the binary from the build stage.
COPY --from=builder /external-dns-technitium-dns /external-dns-technitium-dns

EXPOSE 8888 8080

ENTRYPOINT ["/external-dns-technitium-dns"]
