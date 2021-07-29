OS=linux darwin
ARCHS=arm amd64

build:
	@-for os in $(OS) ; do \
		for arch in $(ARCHS); do \
			GOOS=$$os GOARCH=$$arch go build -ldflags="-s" -o bin/pm-$$os-$$arch ./example; \
		done; \
	done

