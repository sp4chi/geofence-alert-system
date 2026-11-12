# Stage 1: compile with the full Go toolchain.
FROM golang:1.27-alpine AS build
WORKDIR /src

# Download modules first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off → fully static binary that runs on a minimal base image.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Stage 2: only the binary, no shell or package manager, runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
