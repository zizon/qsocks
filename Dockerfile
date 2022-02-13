from golang:1.17-alpine as build

add . /build-work
workdir /build-work
run go mod tidy
run go build -o qsocks

from golang:1.17-alpine
copy --from=build /build-work/qsocks /usr/local/bin/qsocks