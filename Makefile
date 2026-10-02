build-fips:
	GOEXPERIMENT=boringcrypto go build -tags=boringcrypto

clean:
	rm -f tyk-pump

run-fips: build-fips
	./tyk-pump

validate-fips: build-fips
	go tool nm tyk-pump | grep -i boring

# End-to-end OpenTelemetry metrics suite (ci/tests/metrics): promtest unit
# tests, then setup/test/teardown for every profile. Needs docker and Task
# (https://taskfile.dev). Runs against PUMP_IMAGE, default internal/tyk-pump;
# build it with `docker build -t internal/tyk-pump .`.
test-metrics-e2e:
	cd ci/tests/metrics && task

.PHONY: build-fips clean run-fips validate-fips test-metrics-e2e
