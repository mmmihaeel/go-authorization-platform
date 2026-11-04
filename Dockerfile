# syntax=docker/dockerfile:1.9
FROM golang:1.27.3-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN for target in authz-api audit-worker audit-consumer demo-resource-api admission-webhook; do \
      CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o "/out/${target}" "./cmd/${target}"; \
    done

FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGET=authz-api
WORKDIR /app
COPY --from=build "/out/${TARGET}" /app/app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/app"]
