FROM golang:1.21.6-alpine as build

WORKDIR /service
COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 go build -v -o /bin/ledger_service .

# ---

FROM alpine:latest

WORKDIR /
COPY --from=build /bin/ledger_service /ledger_service

ENTRYPOINT ["./ledger_service"]