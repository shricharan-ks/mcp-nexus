# ---------- builder ----------
FROM golang:1.23 AS builder

WORKDIR /workspace

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy only the directories needed for the build
COPY cmd/    cmd/
COPY api/    api/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o manager ./cmd/operator/

# ---------- runtime ----------
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
