NAME ?= fleet
VERSION ?= 0.5.4
NV = $(NAME)-v$(VERSION)
SPEC_FILE := $(NAME).spec
TARBALL := $(NV).tar.gz
SRCDIR ?= ./
DESTDIR ?= ./

.PHONY: srpm

srpm: $(TARBALL) $(SPEC_FILE)
	rpmbuild --define '_sourcedir $(SRCDIR)' --define '_srcrpmdir $(DESTDIR)' -bs $(SPEC_FILE)

$(TARBALL):
	cd ../../ && git archive --format=tar.gz --prefix=$(NAME)-$(VERSION)/ v$(VERSION) -o ${PWD}/$(TARBALL)

clean:
	rm -fv *gz *rpm

build: srpm
	rpmbuild --rebuild $(NAME)*$(VERSION)*.src.rpm
