FROM magneticio/go-pre-dockerimage:v0.0.1 AS build-env
ADD . /root/src
RUN cd /root/src && CGO_ENABLED=0 go build -o vampadapter

FROM alpine:3.9

RUN apk --no-cache add bash curl

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.15.0/bin/linux/amd64/kubectl \
  && chmod +x ./kubectl \
  && mv ./kubectl /usr/local/bin/kubectl

WORKDIR /app
COPY --from=build-env /root/src/vampadapter /app/
COPY --from=build-env /root/src/adapter/resources/setup.yml /app/
COPY --from=build-env /root/src/entrypoint.sh /app/
RUN chmod +x ./entrypoint.sh
ENTRYPOINT ["./entrypoint.sh"]


CMD ["9000"]

EXPOSE 9000
