FROM magneticio/go-pre-dockerimage:v0.0.1 AS build-env
ADD . /root/src
RUN cd /root/src && CGO_ENABLED=0 go build -o vampadapter

FROM alpine:3.9
WORKDIR /app
COPY --from=build-env /root/src/vampadapter /app/
ENTRYPOINT ["./vampadapter"]

CMD ["9000"]

EXPOSE 9000
