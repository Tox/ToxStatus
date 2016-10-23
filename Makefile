all: prep assets
	go build -o build/bin/toxstatus github.com/Tox/ToxStatus/cmd/toxstatus

assets:
	go run vendor/github.com/Impyy/go-embed/*.go -pkg=main -input=cmd/toxstatus/assets -output=cmd/toxstatus/assets.go

prep:
	mkdir -p build/bin

clean:
	rm -rf build
	rm -f cmd/toxstatus/assets.go