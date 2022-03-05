from golang:1.18rc1-alpine as build

add . /build-work
workdir /build-work
run go mod tidy
run go build -o qsocks

from golang:1.18rc1-alpine
copy --from=build /build-work/qsocks /usr/local/bin/qsocks
