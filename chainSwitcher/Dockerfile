FROM golang as build
COPY . /go/src/github.com/btccom/btcpool-go-modules/
RUN bash /go/src/github.com/btccom/btcpool-go-modules/chainSwitcher/build.sh

FROM php:cli
COPY --from=build /go/src/github.com/btccom/btcpool-go-modules/chainSwitcher/chainSwitcher /usr/local/bin/
COPY install/cfg-generator/ /usr/local/bin/
COPY chainSwitcher/entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
