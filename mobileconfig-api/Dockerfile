FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mobileconfig-api .

FROM scratch

ARG CREATED
ARG REVISION
ARG VERSION

LABEL org.opencontainers.image.title="mobileconfig-api" \
      org.opencontainers.image.description="Signs and serves Apple mobileconfig profiles for ACME certificate enrollment and CA trust." \
      org.opencontainers.image.source="https://github.com/caraar12345/go-utilities/tree/main/mobileconfig-api" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.created=$CREATED \
      org.opencontainers.image.revision=$REVISION \
      org.opencontainers.image.version=$VERSION

COPY --from=builder /app/mobileconfig-api /mobileconfig-api

EXPOSE 8765

ENTRYPOINT ["/mobileconfig-api"]
