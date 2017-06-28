FROM alpine:3.6

COPY database-controller /
CMD [ "/database-controller" ]
