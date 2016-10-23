toxstatus: prep
	go build -o build/bin/toxstatus github.com/Tox/ToxStatus/cmd/toxstatus

prep:
	mkdir -p build/bin

clean:
	rm -rf build