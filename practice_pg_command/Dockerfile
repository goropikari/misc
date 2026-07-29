FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /practice-pg-command .
FROM gcr.io/distroless/static-debian12
COPY --from=build /practice-pg-command /practice-pg-command
EXPOSE 5432
ENTRYPOINT ["/practice-pg-command"]
