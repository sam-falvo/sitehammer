INSTALL		:= go install
SITEHAMMER	:= github.com/sam-falvo/sitehammer

all: binaries

binaries:
	${INSTALL} ${SITEHAMMER}/hammer

