FROM golang:1.27.2-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build

RUN apk upgrade --no-cache

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false go build -o /out/payroll-backend .
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false go build -o /out/migrate ./migrate
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false go build -o /out/seed ./seed

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=build /out/payroll-backend /app/payroll-backend
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/seed /app/seed

USER app
EXPOSE 8000

CMD ["/app/payroll-backend"]
