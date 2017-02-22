all: mongodb_exporter

vendor: glide.yaml glide.lock
	glide install

mongodb_exporter: vendor mongodb_exporter.go collector/*.go collector/mongod/*.go collector/mongos/*.go shared/*.go
	go build -o mongodb_exporter mongodb_exporter.go

clean:
	rm -rf mongodb_exporter vendor 2>/dev/null || true
