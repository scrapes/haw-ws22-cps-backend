FROM golang:1.22 as Build
LABEL authors="ans"

RUN mkdir /app && cd /app
WORKDIR /app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -o /build

#--------
FROM alpine:latest

COPY --from=Build /build /app

ENTRYPOINT ["/app"]