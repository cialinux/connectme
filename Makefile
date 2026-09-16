.PHONY: test fmt vet compose-config run
test:
	go test ./...
fmt:
	gofmt -w cmd core migrations modules
vet:
	go vet ./...
compose-config:
	docker compose --env-file .env config --quiet
run:
	docker compose --env-file .env up -d --pull always --wait
