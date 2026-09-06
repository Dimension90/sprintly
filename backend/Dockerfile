FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go test ./... && go build -trimpath -ldflags="-s -w" -o /sprintly-api .

FROM alpine:3.21
RUN addgroup -S sprintly && adduser -S sprintly -G sprintly && mkdir -p /app/data && chown -R sprintly:sprintly /app
USER sprintly
WORKDIR /app
COPY --from=build /sprintly-api /usr/local/bin/sprintly-api
EXPOSE 8080
CMD ["sprintly-api"]
