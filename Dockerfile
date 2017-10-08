FROM golang:1.8 AS build

WORKDIR /go/src/app
COPY . .

RUN env CGO_ENABLED=0 go build -a -o /database-controller

FROM scratch

COPY --from=build /database-controller /database-controller
ENTRYPOINT [ "/database-controller" ]
