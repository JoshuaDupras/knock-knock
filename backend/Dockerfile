# --- build stage ---
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server .

# --- tiny runtime ---
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/server /server
USER nonroot
ENV PORT=3000
EXPOSE 3000
ENTRYPOINT ["/server"]
