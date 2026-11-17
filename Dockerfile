FROM golang:1.26-alpine AS source
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./

FROM source AS build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/counter .

FROM alpine:3.23 AS runtime
RUN addgroup -S counter && adduser -S -G counter counter
WORKDIR /app
COPY --from=build /out/counter ./counter
USER counter
EXPOSE 8080
ENTRYPOINT ["/app/counter"]
