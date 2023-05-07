FROM golang:1.20-alpine AS build

WORKDIR /app
 
COPY ./publisher/go.sum ./
COPY ./publisher/go.mod ./
COPY ./rabbit ../rabbit

RUN go mod download

COPY ./publisher ./

RUN go build -o /publisher

FROM alpine

WORKDIR /

COPY --from=build /publisher /publisher

ENTRYPOINT ["/publisher"]
