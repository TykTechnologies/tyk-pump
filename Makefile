build-fips:
	GOEXPERIMENT=boringcrypto go build -tags=boringcrypto

clean:
	rm -f tyk-pump

run-fips: build-fips
	./tyk-pump

validate-fips: build-fips
	go tool nm tyk-pump | grep -i boring

.PHONY: build-fips clean run-fips validate-fips
