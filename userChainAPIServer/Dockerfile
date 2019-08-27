FROM golang as build
COPY . /go/src/github.com/btccom/btcpool-go-modules/
RUN cd /go/src/github.com/btccom/btcpool-go-modules/userChainAPIServer && go build

FROM php:cli
COPY --from=build /go/src/github.com/btccom/btcpool-go-modules/userChainAPIServer/userChainAPIServer /usr/local/bin/
COPY install/cfg-generator/ /usr/local/bin/
COPY userChainAPIServer/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
