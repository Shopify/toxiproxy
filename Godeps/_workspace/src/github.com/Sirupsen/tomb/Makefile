include $(GOROOT)/src/Make.inc

all: package

TARG=launchpad.net/tomb

GOFILES=\
	tomb.go\

GOFMT=gofmt
BADFMT:=$(shell $(GOFMT) -l $(GOFILES) $(CGOFILES) $(wildcard *_test.go) 2> /dev/null)

gofmt: $(BADFMT)
	@for F in $(BADFMT); do $(GOFMT) -w $$F && echo $$F; done

ifneq ($(BADFMT),)
ifneq ($(MAKECMDGOALS),gofmt)
$(warning WARNING: make gofmt: $(BADFMT))
endif
endif

include $(GOROOT)/src/Make.pkg
