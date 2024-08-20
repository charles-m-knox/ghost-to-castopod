test:
	go test -test.v -coverprofile=testcov.out ./... && \
	go tool cover -html=testcov.out
