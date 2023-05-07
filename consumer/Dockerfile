FROM golang:1.20-alpine AS build

WORKDIR /app
 
COPY ./consumer/go.sum ./
COPY ./consumer/go.mod ./
COPY ./rabbit ../rabbit

RUN go mod download

COPY ./consumer ./

RUN go build -o /consumer

FROM alpine

WORKDIR /

COPY --from=build /consumer /consumer

ENTRYPOINT ["/consumer"]
