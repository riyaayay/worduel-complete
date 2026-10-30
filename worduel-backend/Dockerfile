# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /worduel-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /worduel-server /worduel-server
EXPOSE 8080
ENV PORT=8080
ENV ENVIRONMENT=production
USER nonroot:nonroot
ENTRYPOINT ["/worduel-server"]
