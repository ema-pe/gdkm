.PHONY: lint

gdkm : main.go go.mod go.sum
	go build ./...

lint :
	go vet ./...
	staticcheck ./...

clean :
	rm --force gdkm
