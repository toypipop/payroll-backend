FROM golang:1.27.2-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false go build -o /out/payroll-backend .
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false go build -o /out/migrate ./migrate
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false go build -o /out/seed ./seed

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=build /out/payroll-backend /app/payroll-backend
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/seed /app/seed

USER app
EXPOSE 8000

CMD ["/app/payroll-backend"]
