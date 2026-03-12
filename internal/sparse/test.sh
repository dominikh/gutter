#!/bin/sh
trap "exit 1" SIGINT

# run <goarch> <goexperiment> <tags> <goamd64> <args>
function run() {
		echo "GOEXPERIMENT=$2 GOAMD64=$4 GOARCH=$1 -tags=$3:"
		GOEXPERIMENT=$2 GOAMD64=$4 GOARCH=$1 go test -tags="$3" -short "$5" || exit $?
		echo
}

for tags in noasm ""; do
	for exp in simd ""; do
		for amd64 in v2 v3; do
			run amd64 "$exp" "$tags" "$amd64" "$@"
		done
	done
done

for tags in noasm ""; do
	for arch in 386 arm64; do
		for exp in simd ""; do
			run "$arch" "$exp" "$tags" "" "$@"
		done
	done
done

