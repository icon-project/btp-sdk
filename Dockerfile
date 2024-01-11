# syntax=docker/dockerfile:1
FROM alpine:3.18 as build
RUN apk add git make go
RUN git clone https://github.com/icon-project/btp-sdk.git /src
WORKDIR /src
RUN make linux

FROM alpine:3.18
WORKDIR /
RUN mkdir /btp-sdk
COPY --from=build /src/build/linux/btp-sdk-cli /btp-sdk
CMD ./btp-sdk/btp-sdk-cli server start
