FROM scratch
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/stformat /usr/bin/stformat
ENTRYPOINT ["/usr/bin/stformat"]