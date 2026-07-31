FROM golang:1.26.5 AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/fgalens .

FROM alpine:3.23.5
RUN apk add --no-cache ca-certificates
COPY --from=build /out/fgalens /fgalens
COPY demo/lens-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
