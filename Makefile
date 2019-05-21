mqtt-forwarder:
	go build -o build/$@ -ldflags='-s -w' -mod=vendor ./cmd
